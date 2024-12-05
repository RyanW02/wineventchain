use std::{future::Future, sync::Arc, time::Duration};

use crate::{AgentError, DiskQueue, DiskQueueOptions, Result};
use log::{error, info, warn};
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use time::OffsetDateTime;
use tokio::time::sleep;

pub struct RetryQueue<T, U, S, RetryF, RetryFut, SuccessF, SuccessFut>
where
    T: Clone + DeserializeOwned + Serialize + Send + Sync + 'static,
    U: Send + Sync + 'static,
    S: Send + Sync + 'static,
    RetryF: Fn(Arc<S>, T) -> RetryFut + Send + Sync + 'static,
    RetryFut: Future<Output = Result<U>> + Send + 'static,
    SuccessF: Fn(Arc<S>, U) -> SuccessFut + Send + Sync + 'static,
    SuccessFut: Future<Output = ()> + Send + 'static,
{
    queue: Arc<DiskQueue<RetryItem<T>>>,
    options: Arc<RetryOptions>,
    state: Arc<S>,
    retry_handler: Option<RetryF>,
    success_callback: Option<SuccessF>,
}

pub struct RetryOptions {
    pub queue_name: String,
    pub max_queue_size: Option<usize>,
    pub backoff: Vec<Duration>,
    pub check_interval: Duration,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
struct RetryItem<T: Clone> {
    first_attempt: OffsetDateTime,
    last_attempt: OffsetDateTime,
    attempts: u32,
    item: T,
}

impl<T, U, S, RetryF, RetryFut, SuccessF, SuccessFut>
    RetryQueue<T, U, S, RetryF, RetryFut, SuccessF, SuccessFut>
where
    T: Clone + DeserializeOwned + Serialize + Send + Sync + 'static,
    U: Send + Sync + 'static,
    S: Send + Sync + 'static,
    RetryF: Fn(Arc<S>, T) -> RetryFut + Send + Sync + 'static,
    RetryFut: Future<Output = Result<U>> + Send + 'static,
    SuccessF: Fn(Arc<S>, U) -> SuccessFut + Send + Sync + 'static,
    SuccessFut: Future<Output = ()> + Send + 'static,
{
    pub fn new(
        db: Arc<sled::Db>,
        options: RetryOptions,
        state: Arc<S>,
        retry_handler: RetryF,
        success_callback: SuccessF,
    ) -> Result<Self> {
        Ok(Self {
            queue: Arc::new(DiskQueue::new(
                db,
                DiskQueueOptions {
                    tree_name: options.queue_name.clone(),
                    max_size: options.max_queue_size,
                },
            )?),
            options: Arc::new(options),
            state,
            retry_handler: Some(retry_handler),
            success_callback: Some(success_callback),
        })
    }

    pub fn push(&self, item: T) -> Result<u64> {
        let retry_item = RetryItem {
            first_attempt: OffsetDateTime::now_utc(),
            last_attempt: OffsetDateTime::now_utc(),
            attempts: 0,
            item,
        };

        self.queue.push(retry_item)
    }

    pub fn start_retry_loop(&mut self) -> Result<()> {
        if self.options.backoff.is_empty() {
            warn!("No backoff durations provided, so retry loop will not run");
            return Ok(());
        }

        let queue = Arc::clone(&self.queue);
        let options = Arc::clone(&self.options);
        let state = Arc::clone(&self.state);
        let retry_handler = self
            .retry_handler
            .take()
            .ok_or(AgentError::RetryQueueAlreadyStarted)?;
        let success_callback = self
            .success_callback
            .take()
            .ok_or(AgentError::RetryQueueAlreadyStarted)?;

        tokio::spawn(async move {
            loop {
                sleep(options.check_interval).await;
                info!("Checking retry queue");

                for retry_item in queue.iter() {
                    let (key, retry_item) = match retry_item {
                        Ok(item) => item,
                        Err(e) => {
                            error!("Error reading from retry queue: {}", e);
                            continue;
                        }
                    };

                    let interval = match options.backoff.get(retry_item.attempts as usize) {
                        Some(interval) => *interval,
                        None => {
                            warn!("Retry attempts exhausted for item with key {}", key);
                            queue.remove(key).unwrap();
                            continue;
                        }
                    };

                    if retry_item.last_attempt + interval > OffsetDateTime::now_utc() {
                        continue;
                    }

                    info!("Found item with key {} due a retry", key);

                    match Self::attempt_retry_and_update(
                        queue.as_ref(),
                        options.as_ref(),
                        Arc::clone(&state),
                        &retry_handler,
                        key,
                        retry_item.clone(),
                    )
                    .await
                    {
                        Ok(res) => {
                            info!("Callback succeeded for item with key {}", key);
                            tokio::spawn(success_callback(Arc::clone(&state), res));
                        }
                        Err(e) => error!("Error attempting retry for item with key {}: {}", key, e),
                    }
                }
            }
        });

        Ok(())
    }

    async fn attempt_retry_and_update(
        queue: &DiskQueue<RetryItem<T>>,
        options: &RetryOptions,
        state: Arc<S>,
        retry_handler: &RetryF,
        key: u64,
        mut item: RetryItem<T>,
    ) -> Result<U> {
        match retry_handler(state, item.item.clone()).await {
            Ok(res) => {
                info!("Retry attempt succeeded for item with key {}", key);
                queue.remove(key)?;
                Ok(res)
            }
            Err(e) => {
                warn!("Retry attempt failed for item with key {}: {}", key, e);
                item.last_attempt = OffsetDateTime::now_utc();
                item.attempts += 1;

                if item.attempts < options.backoff.len() as u32 {
                    info!("Will retry item with key {} again later", key);
                    queue.update(key, item)?;
                } else {
                    warn!("Retry attempts exhausted for item with key {}", key);
                    queue.remove(key)?;
                }

                Err(e)
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use parking_lot::Mutex;
    use tempdir::TempDir;

    use super::*;

    #[tokio::test]
    async fn test_retry_queue() -> Result<()> {
        let success = Arc::new(Mutex::new(false));

        let mut queue = build_queue(
            // Instantly remove if now successful, but must define at least one more backoff to stop removal
            // of still-failing tasks due to exhausted attempts.
            vec![Duration::ZERO, Duration::from_secs(5)],
            Arc::clone(&success),
            |_, item| async move {
                if item == "fail" {
                    Err(AgentError::Other("fail".to_string()))
                } else {
                    Ok(item)
                }
            },
            |success, item| async move {
                assert_eq!(item, "success");
                *success.lock() = true;
            },
        );

        queue.start_retry_loop()?;

        queue.push("success".to_string())?;
        queue.push("fail".to_string())?;

        sleep(Duration::from_millis(50)).await;

        let remaining = queue.queue.iter().collect::<Result<Vec<_>>>()?;
        assert_eq!(remaining.len(), 1);

        assert_eq!(*success.lock(), true);

        Ok(())
    }

    #[tokio::test]
    async fn test_exhaust_attempts() -> Result<()> {
        let mut queue = build_queue(
            vec![Duration::from_millis(30)],
            Arc::new(()),
            |_, _| async move { Err(AgentError::Other("fail".to_string())) },
            |_, _: ()| async move {},
        );

        queue.start_retry_loop()?;

        queue.push("fail".to_string())?;
        queue.push("fail".to_string())?;
        queue.push("fail".to_string())?;

        sleep(Duration::from_millis(55)).await;

        let remaining = queue.queue.iter().collect::<Result<Vec<_>>>()?;
        assert_eq!(remaining.len(), 0);

        Ok(())
    }

    fn build_queue<T, U, S, RetryF, RetryFut, SuccessF, SuccessFut>(
        backoff: Vec<Duration>,
        state: Arc<S>,
        retry_handler: RetryF,
        success_callback: SuccessF,
    ) -> RetryQueue<T, U, S, RetryF, RetryFut, SuccessF, SuccessFut>
    where
        T: Clone + DeserializeOwned + Serialize + Send + Sync + 'static,
        U: Send + Sync + 'static,
        S: Send + Sync + 'static,
        RetryF: Fn(Arc<S>, T) -> RetryFut + Send + Sync + 'static,
        RetryFut: Future<Output = Result<U>> + Send + 'static,
        SuccessF: Fn(Arc<S>, U) -> SuccessFut + Send + Sync + 'static,
        SuccessFut: Future<Output = ()> + Send + 'static,
    {
        let tmp = TempDir::new("sled").expect("tempdir failed");
        let db = sled::open(tmp.path().join("test.db")).expect("sled::open failed");

        RetryQueue::new(
            Arc::new(db),
            RetryOptions {
                queue_name: "test".to_string(),
                max_queue_size: Some(100),
                backoff,
                check_interval: Duration::from_millis(10),
            },
            state,
            retry_handler,
            success_callback,
        )
        .expect("RetryQueue::new failed")
    }
}
