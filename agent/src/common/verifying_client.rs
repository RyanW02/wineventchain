use crate::{AgentError, Result};
use async_trait::async_trait;
use backoff::ExponentialBackoff;
use core::fmt;
use log::{error, warn};
use rand::prelude::SliceRandom;
use serde::{Deserialize, Serialize};
use std::future::Future;
use std::sync::Arc;
use std::time::Duration;
use tokio::task::JoinSet;
use tokio::time::timeout;

pub struct VerifyingClient<T, U>
where
    T: Clone + Send + Sync + 'static,
    U: Tester<Item = T>,
{
    // Tuple of alive clients and dead clients
    clients: Vec<PooledClient<T>>,
    tester: U,
    options: VerifyingClientOptions,
}

#[async_trait]
pub trait Tester {
    type Item;
    async fn test(&self, client: &Self::Item) -> bool;
}

#[derive(Clone, Debug)]
pub struct PooledClient<T: Clone> {
    id: u64,
    pub client: T,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct VerifyingClientOptions {
    pub health_check_timeout: Duration,
    pub verification_client_count: usize,
    pub minimum_verification_client_count: usize,
    pub allow_self_verification: bool,
    pub max_resubmits: usize,
    pub failure_strategy: FailureStrategy,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[allow(dead_code)]
pub enum FailureStrategy {
    Error,
    AcceptWithNSuccesses(usize),
}

impl<T: Clone> PartialEq for PooledClient<T> {
    fn eq(&self, other: &Self) -> bool {
        self.id == other.id
    }
}

impl Default for VerifyingClientOptions {
    fn default() -> Self {
        Self {
            health_check_timeout: Duration::from_secs(5),
            verification_client_count: 3,
            minimum_verification_client_count: 1,
            allow_self_verification: true,
            max_resubmits: 0,
            failure_strategy: FailureStrategy::Error,
        }
    }
}

impl<T, U> VerifyingClient<T, U>
where
    T: Clone + Send + Sync + 'static,
    U: Tester<Item = T>,
{
    pub fn new(clients: Vec<T>, tester: U, options: VerifyingClientOptions) -> Self {
        let clients_with_id = clients
            .into_iter()
            .enumerate()
            .map(|(id, client)| PooledClient {
                id: id as u64,
                client,
            })
            .collect();

        Self {
            clients: clients_with_id,
            tester,
            options,
        }
    }

    pub async fn get(&self, excluding: &[&PooledClient<T>]) -> Option<PooledClient<T>> {
        self.get_n(1, excluding).await.into_iter().nth(0)
    }

    pub async fn get_n(&self, n: usize, excluding: &[&PooledClient<T>]) -> Vec<PooledClient<T>> {
        let mut filtered: Vec<PooledClient<T>> = self
            .clients
            .iter()
            .filter(|c: &&PooledClient<T>| !excluding.contains(c))
            .cloned()
            .collect::<Vec<_>>();

        if filtered.is_empty() {
            return vec![];
        }

        // Randomise order
        filtered.shuffle(&mut rand::thread_rng());

        let mut clients = Vec::with_capacity(n);
        for client in filtered {
            let is_alive = timeout(
                self.options.health_check_timeout,
                self.tester.test(&client.client),
            )
            .await
            .unwrap_or(false);

            if is_alive {
                clients.push(client);
            }

            if clients.len() >= n {
                break;
            }
        }

        clients
    }

    pub async fn run<V, Fut, E, VerifierFut>(
        &self,
        backoff_config: ExponentialBackoff,
        task: impl Fn(T) -> Fut,
        verifier: fn(T, Arc<V>) -> VerifierFut,
    ) -> Result<Arc<V>>
    where
        V: Send + Sync + 'static,
        Fut: Future<Output = Result<V>>,
        E: fmt::Debug + Send + 'static,
        VerifierFut: Future<Output = std::result::Result<bool, backoff::Error<E>>> + Send + 'static,
    {
        let mut retries = self.options.max_resubmits + 1;
        while retries > 0 {
            match self.run_once(backoff_config.clone(), &task, verifier).await {
                Ok(res) => return Ok(res),
                Err(AgentError::VerificationFailed) => {
                    error!("Verification failed, resubmitting...");
                    retries -= 1;
                }
                Err(e) => return Err(e),
            }
        }

        Err(AgentError::VerificationFailed)
    }

    async fn run_once<V, Fut, E, VerifierFut>(
        &self,
        backoff_config: ExponentialBackoff,
        task: impl Fn(T) -> Fut,
        verifier: fn(T, Arc<V>) -> VerifierFut,
    ) -> Result<Arc<V>>
    where
        V: Send + Sync + 'static,
        Fut: Future<Output = Result<V>>,
        E: fmt::Debug + Send + 'static,
        VerifierFut: Future<Output = std::result::Result<bool, backoff::Error<E>>> + Send + 'static,
    {
        let client: PooledClient<T> = self.get(&[]).await.ok_or(AgentError::NoClientsAvailable)?;
        let res = Arc::new(task(client.client.clone()).await?);

        // Initial interval in the backoff_config will wait first before starting verification.

        // Select verification clients
        let mut verification_clients: Vec<PooledClient<T>> = self
            .get_n(self.options.verification_client_count, &[&client])
            .await;

        if verification_clients.len() < self.options.verification_client_count {
            // If we don't have enough clients, check if we can use the same client
            if self.options.allow_self_verification {
                verification_clients.push(client.clone());
            }

            // If we don't have the hard minimum amount of clients, return an error
            if verification_clients.len() < self.options.minimum_verification_client_count {
                return Err(AgentError::NotEnoughClientsAvailable(
                    verification_clients.len(),
                    self.options.minimum_verification_client_count,
                ));
            }

            // Check size again as we may have added the self client.
            if verification_clients.len() < self.options.verification_client_count {
                warn!(
                    "Not enough clients available for verification, but continuing as minimum threshold met: {} < {}, but {} >= {}",
                    verification_clients.len(),
                    self.options.verification_client_count,
                    verification_clients.len(),
                    self.options.minimum_verification_client_count,
                );
            }
        }

        let verification_client_count = verification_clients.len();

        // Verify the result
        let mut set = JoinSet::new();
        for client in verification_clients {
            let res = Arc::clone(&res);
            let backoff_config = backoff_config.clone();

            set.spawn(async move {
                let res = backoff::future::retry(backoff_config, || async {
                    verifier(client.client.clone(), Arc::clone(&res)).await
                })
                .await;
                (client, res.is_ok() && res.unwrap())
            });
        }

        let mut success_count = 0;
        while let Some(res) = set.join_next().await {
            if res.is_ok() && res.unwrap().1 {
                success_count += 1;
            }
        }

        // If still failing after retrying, follow failure strategy
        if success_count < verification_client_count {
            error!(
                "Verification still failed after retrying ({} failed)",
                verification_client_count - success_count
            );

            match self.options.failure_strategy {
                FailureStrategy::Error => Err(AgentError::VerificationFailed)?,
                FailureStrategy::AcceptWithNSuccesses(n) => {
                    if success_count < n {
                        Err(AgentError::VerificationFailed)?
                    }
                }
            }
        }

        Ok(res)
    }
}

#[cfg(test)]
mod tests {
    use std::time::Duration;

    use super::*;
    use backoff::{exponential::ExponentialBackoff, Error, ExponentialBackoffBuilder, SystemClock};
    use parking_lot::Mutex;

    type Result<T> = std::result::Result<T, Error<AgentError>>;

    #[derive(Clone, Debug)]
    struct TestClient {
        is_alive: Arc<Mutex<bool>>,
    }

    impl PartialEq for TestClient {
        fn eq(&self, other: &Self) -> bool {
            Arc::ptr_eq(&self.is_alive, &other.is_alive)
        }
    }

    struct MockTester;

    #[async_trait]
    impl Tester for MockTester {
        type Item = TestClient;

        async fn test(&self, client: &TestClient) -> bool {
            *client.is_alive.lock()
        }
    }

    impl TestClient {
        pub fn new(is_alive: bool) -> Self {
            Self {
                is_alive: Arc::new(Mutex::new(is_alive)),
            }
        }
    }

    fn fast_backoff() -> ExponentialBackoff<SystemClock> {
        ExponentialBackoffBuilder::new()
            .with_initial_interval(Duration::from_millis(25))
            .with_multiplier(1.0)
            .with_max_interval(Duration::from_millis(50))
            .with_max_elapsed_time(Some(Duration::from_millis(500)))
            .build()
    }

    #[tokio::test]
    async fn test_get_one() {
        let clients = vec![TestClient::new(true)];

        let client = VerifyingClient::new(
            clients.clone(),
            MockTester {},
            VerifyingClientOptions::default(),
        );
        let retrieved = client.get(&[]).await.unwrap().client;

        assert_eq!(retrieved, clients[0]);
    }

    #[tokio::test]
    async fn get_multiple_distinct() {
        let clients = vec![
            TestClient::new(true),
            TestClient::new(true),
            TestClient::new(true),
        ];

        let client = VerifyingClient::new(
            clients.clone(),
            MockTester {},
            VerifyingClientOptions::default(),
        );
        let retrieved = client.get_n(3, &[]).await;

        // Assert distinct
        assert_eq!(retrieved.len(), 3);

        assert!(retrieved[0] != retrieved[1], "Expected distinct clients");
        assert!(retrieved[0] != retrieved[2], "Expected distinct clients");
        assert!(retrieved[1] != retrieved[2], "Expected distinct clients");

        // Assert all clients are in the original list
        assert!(clients.contains(&retrieved[0].client));
        assert!(clients.contains(&retrieved[1].client));
        assert!(clients.contains(&retrieved[2].client));
    }

    #[tokio::test]
    async fn test_get_more_than_max() {
        let clients = vec![TestClient::new(true), TestClient::new(true)];

        let client = VerifyingClient::new(
            clients.clone(),
            MockTester {},
            VerifyingClientOptions::default(),
        );
        let retrieved = client.get_n(3, &[]).await;

        assert_eq!(retrieved.len(), 2);
    }

    #[tokio::test]
    async fn test_run_success() {
        let clients = vec![TestClient::new(true), TestClient::new(true)];

        let client = VerifyingClient::new(clients.clone(), MockTester {}, Default::default());

        let res = client
            .run(
                fast_backoff(),
                |_| async move { Ok(true) },
                |_, _| async move { Result::Ok(true) },
            )
            .await
            .unwrap();

        assert_eq!(res, true.into());
    }

    #[tokio::test]
    async fn test_run_failure() {
        let clients = vec![TestClient::new(true), TestClient::new(true)];

        let client = VerifyingClient::new(clients.clone(), MockTester {}, Default::default());

        let res = client
            .run(
                fast_backoff(),
                |_| async move { Ok(true) },
                |_, _| async move { Result::Ok(false) },
            )
            .await;

        assert!(res.is_err(), "Expected an error");
        match res.unwrap_err() {
            AgentError::VerificationFailed => {}
            e => panic!("Expected a verification failure, got {:?}", e),
        }
    }

    #[tokio::test]
    async fn test_run_failure_retry() {
        let clients = vec![TestClient::new(true), TestClient::new(true)];

        let client = VerifyingClient::new(clients.clone(), MockTester {}, Default::default());

        let res = client
            .run(
                fast_backoff(),
                |_| async move { Ok(true) },
                |client: TestClient, _| async move {
                    let mut alive = client.is_alive.lock();
                    let old = *alive;
                    *alive = !*alive;
                    Result::Ok(old)
                },
            )
            .await
            .unwrap();

        assert_eq!(res, true.into());
    }
}
