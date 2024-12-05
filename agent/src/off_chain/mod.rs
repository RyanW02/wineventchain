mod http_client;
mod retry_queue;
pub use retry_queue::{start_retry_loop, QueuedOffChainEvent};

use std::{sync::Arc, time::Duration};

use crate::{
    common::{FailureStrategy, VerifyingClient, VerifyingClientOptions},
    model::{events::EventData, rpc::HexBytes},
    AgentError, Config, Result,
};

use backoff::{Error, ExponentialBackoffBuilder};
use http_client::HttpClient;
use log::{error, warn};
use tendermint::Hash;

use self::http_client::HttpClientTester;

pub struct OffChainClient {
    client: VerifyingClient<HttpClient, HttpClientTester>,
}

impl OffChainClient {
    pub fn new(config: Arc<Config>) -> Self {
        let clients = config
            .off_chain
            .nodes
            .iter()
            .map(|node| HttpClient::new_with_client(
                Arc::clone(&config),
                node.http_endpoint.clone(),
                reqwest::ClientBuilder::new()
                    .timeout(Duration::from_secs(5))
                    .build()
                    .expect("Failed to build reqwest client"), // Should be infallible
            ))
            .collect::<Vec<_>>();

        let options = VerifyingClientOptions {
            health_check_timeout: Duration::from_secs(
                config.off_chain.verification.health_check_timeout_seconds,
            ),
            verification_client_count: config.off_chain.verification.verification_client_count,
            minimum_verification_client_count: config
                .off_chain
                .verification
                .minimum_verification_client_count,
            allow_self_verification: config.off_chain.verification.allow_self_verification,
            max_resubmits: config.off_chain.verification.max_resubmits,
            failure_strategy: FailureStrategy::Error,
        };

        let client = VerifyingClient::new(clients, HttpClientTester {}, options);
        Self { client }
    }

    pub async fn health(&self) -> bool {
        self.client.get(&[]).await.is_some()
    }

    pub async fn submit(
        &self,
        event_id: HexBytes,
        tx_hash: Hash,
        event_data: EventData,
    ) -> Result<()> {
        let backoff_config = ExponentialBackoffBuilder::new().build();

        self.client
            .run(
                backoff_config,
                |c| {
                    let event_id = event_id.clone();
                    let event_data = event_data.clone();
                    async move {
                        c.store_event(event_id.clone(), tx_hash, event_data.clone())
                            .await?;
                        Ok((event_id, event_data))
                    }
                },
                |c, passed| async move {
                    let event_id = passed.0.clone();
                    let event_data = &passed.1;

                    let res = match c.get_event_data(event_id).await {
                        Ok(Some(res)) => res,
                        Ok(None) => {
                            return Err(Error::transient(AgentError::Other(
                                "event not found".to_string(),
                            )))
                        }
                        Err(err) => {
                            error!("Error getting event data: {}", err);
                            return Err(Error::permanent(err));
                        }
                    };

                    // Compare event data
                    if res.event.event_data != *event_data {
                        warn!(
                            "Event data mismatch, expected: {:?}, got: {:?}",
                            event_data, res.event.event_data
                        );
                        return Err(Error::permanent(AgentError::Other(
                            "event data mismatch".to_string(),
                        )));
                    }

                    Ok(true)
                },
            )
            .await?;

        Ok(())
    }
}
