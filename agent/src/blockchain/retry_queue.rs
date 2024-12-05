use std::sync::Arc;

use crate::{
    common::util,
    model::events::EventWithData,
    off_chain::{OffChainClient, QueuedOffChainEvent},
    Config, Result, RetryOptions, RetryQueue,
};
use log::{error, info};
use tokio::sync::mpsc::{self, UnboundedSender};

use super::BlockchainClient;

struct State {
    blockchain_client: Arc<BlockchainClient>,
    offchain_client: Arc<OffChainClient>,
    offchain_retry_tx: UnboundedSender<QueuedOffChainEvent>,
}

const RETRY_QUEUE_NAME: &str = "retry_blockchain";

pub fn start_retry_loop(
    config: &Config,
    db: Arc<sled::Db>,
    blockchain_client: Arc<BlockchainClient>,
    offchain_client: Arc<OffChainClient>,
    offchain_retry_tx: UnboundedSender<QueuedOffChainEvent>,
) -> Result<UnboundedSender<EventWithData>> {
    let (tx, mut rx) = mpsc::unbounded_channel::<EventWithData>();

    let state = State {
        blockchain_client,
        offchain_client,
        offchain_retry_tx,
    };

    let mut queue = RetryQueue::new(
        db,
        RetryOptions {
            queue_name: RETRY_QUEUE_NAME.to_string(),
            max_queue_size: config.blockchain.max_retry_queue_size,
            backoff: util::parse_durations(config.blockchain.retry_intervals.iter())?,
            check_interval: duration_str::parse(config.blockchain.check_interval.as_str())?,
        },
        Arc::new(state),
        |state, item: EventWithData| async move {
            match state.blockchain_client.submit(item.clone().into()).await {
                Ok((metadata, tx_hash)) => Ok((metadata, tx_hash, item)),
                Err(e) => {
                    error!("Failed to submit event off-chain on retry: {}", e);
                    Err(e)
                }
            }
        },
        |state, (metadata, tx_hash, event)| async move {
            info!(
                "Stored event {} on the blockchain successfully after retry.",
                metadata.event_id
            );

            // Now that the event is stored on the blockchain, submit to off-chain
            match state
                .offchain_client
                .submit(metadata.event_id.clone(), tx_hash, event.event_data.clone())
                .await
            {
                Ok(_) => info!(
                    "Stored event {} off-chain successfully after retry.",
                    metadata.event_id
                ),
                Err(e) => {
                    error!("Failed to store event {} off-chain after successfully storing on blockchain retry. Adding to the retry list. Error: {}", metadata.event_id, e);

                    let queued_event = QueuedOffChainEvent::new(
                        metadata.event_id.clone(),
                        tx_hash,
                        event.event_data,
                    );
                    match state.offchain_retry_tx.send(queued_event) {
                        Ok(_) => info!(
                            "Added event {} to the off-chain retry queue after successfully storing on blockchain retry.",
                            metadata.event_id
                        ),
                        Err(e) => error!(
                            "Failed to add event {} to the off-chain retry queue, after successfulyl storing on blockchain retry: {}",
                            metadata.event_id, e
                        ),
                    }
                }
            }
        },
    )?;
    queue.start_retry_loop()?;

    // Start channel listener
    tokio::spawn(async move {
        while let Some(msg) = rx.recv().await {
            // Clone variables for use in logging
            let channel = msg.system.channel.clone();
            let event_id = msg.system.event_id;

            match queue.push(msg) {
                Ok(_) => info!(
                    "Event {}:{} added to the blockchain retry queue",
                    channel, event_id
                ),
                Err(e) => error!(
                    "Failed to add event {}:{} to the blockchain retry queue: {}",
                    channel, event_id, e
                ),
            }
        }
    });

    Ok(tx)
}
