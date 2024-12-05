use std::sync::Arc;

use log::{error, info};
use serde::{Deserialize, Serialize};
use tendermint::Hash;
use tokio::sync::mpsc::{self, UnboundedSender};

use crate::{
    common::util,
    model::{events::EventData, rpc::HexBytes},
    Config, Result, RetryOptions, RetryQueue,
};

use super::OffChainClient;

struct State {
    client: Arc<OffChainClient>,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct QueuedOffChainEvent {
    event_id: HexBytes,
    tx_hash: Hash,
    event_data: EventData,
}

const RETRY_QUEUE_NAME: &str = "retry_off_chain";

impl QueuedOffChainEvent {
    pub fn new(event_id: HexBytes, tx_hash: Hash, event_data: EventData) -> Self {
        Self {
            event_id,
            tx_hash,
            event_data,
        }
    }
}

pub fn start_retry_loop(
    config: &Config,
    db: Arc<sled::Db>,
    offchain_client: Arc<OffChainClient>,
) -> Result<UnboundedSender<QueuedOffChainEvent>> {
    let (tx, mut rx) = mpsc::unbounded_channel::<QueuedOffChainEvent>();

    let state = State {
        client: offchain_client,
    };

    let mut queue = RetryQueue::new(
        db,
        RetryOptions {
            queue_name: RETRY_QUEUE_NAME.to_string(),
            max_queue_size: config.off_chain.max_retry_queue_size,
            backoff: util::parse_durations(&config.off_chain.retry_intervals)?,
            check_interval: duration_str::parse(config.off_chain.check_interval.as_str())?,
        },
        Arc::new(state),
        |state, item: QueuedOffChainEvent| async move {
            match state
                .client
                .submit(item.event_id.clone(), item.tx_hash, item.event_data.clone())
                .await
            {
                Ok(_) => Ok(item.event_id),
                Err(e) => {
                    error!(
                        "Failed to store event {} on retry. Error: {}",
                        item.event_id, e
                    );

                    Err(e)
                }
            }
        },
        |_, event_id| async move {
            info!(
                "Stored event {} off-chain successfully after retry.",
                event_id
            );
        },
    )?;
    queue.start_retry_loop()?;

    // Start channel listener
    tokio::spawn(async move {
        while let Some(msg) = rx.recv().await {
            let event_id = msg.event_id.clone();

            match queue.push(msg) {
                Ok(_) => info!("Event {} added to the off-chain retry queue", event_id),
                Err(e) => error!(
                    "Failed to add event {} to the off-chain retry queue: {}",
                    event_id, e
                ),
            }
        }
    });

    Ok(tx)
}
