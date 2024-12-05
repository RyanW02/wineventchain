use agent::blockchain::{self, BlockchainClient};
use agent::collector::ChannelCollector;
use agent::model::events::EventWithData;
use agent::off_chain::{self, OffChainClient, QueuedOffChainEvent};
use agent::state::StateStore;
use agent::{Config, Result};
use log::{debug, error, info, trace, warn};
use std::sync::Arc;
use tokio::signal::ctrl_c;
use tokio::sync::mpsc;

#[tokio::main]
pub async fn main() -> Result<()> {
    pretty_env_logger::formatted_builder()
        .filter_module("agent", log::LevelFilter::Info)
        .init();

    let config = Arc::new(Config::load("config.toml").await?);
    info!("Loaded config");

    if config.collector.channels.is_empty() {
        error!("No channels to collect: set `collector.channels` in config.toml");
        return Ok(());
    }

    // Set up Sled state DB
    let db = Arc::new(
        sled::Config::default()
            .path(config.collector.state_db_path.as_str())
            // Be conservative with ID address space, since Sled often thinks we have recovered
            // from a crash on a normal start-up. Default value is 1_000_000.
            .idgen_persist_interval(10_000)
            .open()?,
    );
    // let db = Arc::new(sled::open(config.collector.state_db_path.as_str())?);
    let state_store = Arc::new(StateStore::new(Arc::clone(&db)));

    // Set up blockchain client
    let blockchain_client = Arc::new(BlockchainClient::new(Arc::clone(&config)));

    info!("Testing blockchain health");
    match Arc::clone(&blockchain_client).health().await {
        true => info!("Blockchain connection established!"),
        false => {
            warn!("Blockchain health check failed - will continue to collect events, but will not be \
                able to submit them to the blockchain until the connection is re-established.");
        }
    }

    // Set up offchain client
    let offchain_client = Arc::new(OffChainClient::new(Arc::clone(&config)));

    info!("Testing off-chain interface health");
    match offchain_client.health().await {
        true => info!("Off-chain connection established!"),
        false => {
            warn!(
                "Off-chain health check failed - will continue to collect events, but will not be \
                able to submit them off-chain until the connection is re-established."
            );
        }
    }

    // Set up disk-based retry queues
    let offchain_retry_tx =
        off_chain::start_retry_loop(&config, Arc::clone(&db), Arc::clone(&offchain_client))?;
    let blockchain_retry_tx = blockchain::start_retry_loop(
        &config,
        Arc::clone(&db),
        Arc::clone(&blockchain_client),
        Arc::clone(&offchain_client),
        offchain_retry_tx.clone(),
    )?;

    // Initialise the event channel - this is a global channel that all collectors will write to.
    // It must be global, as it is not possible to pass state to an `extern` function.
    let mut rx = ChannelCollector::init_callback();

    let mut collectors = Vec::with_capacity(config.collector.channels.len());
    for channel in &config.collector.channels {
        info!("Collecting channel: {:?}", channel);
        let mut collector = ChannelCollector::new(*channel);
        unsafe {
            collector.collect(&config).unwrap();
        }
        collectors.push(collector);
    }

    // Spawn an OS thread to listen
    tokio::spawn(async move {
        info!("Starting event loop");

        loop {
            match rx.recv().await {
                Some(ev) => {
                    // Call event handler
                    let state_store = Arc::clone(&state_store);
                    let blockchain_client = Arc::clone(&blockchain_client);
                    let offchain_client = Arc::clone(&offchain_client);
                    let blockchain_retry_tx = blockchain_retry_tx.clone();
                    let offchain_retry_tx = offchain_retry_tx.clone();

                    tokio::spawn(async move {
                        handle_event(
                            ev,
                            &state_store,
                            blockchain_client.as_ref(),
                            offchain_client.as_ref(),
                            blockchain_retry_tx,
                            offchain_retry_tx,
                        )
                        .await;
                    });
                }
                None => {
                    warn!("Event channel received closed, leaving loop.");
                    break;
                }
            }
        }
    });

    ctrl_c().await?;
    info!("Received Ctrl-C!");

    for collector in collectors {
        info!("Shutting down event channel {:?}", collector.channel);

        unsafe {
            if let Err(e) = collector.close() {
                error!("Failed to close collector: {}", e);
            }
        }
    }

    db.flush_async().await?;

    Ok(())
}

async fn handle_event(
    ev: EventWithData,
    state_store: &StateStore,
    blockchain_client: &BlockchainClient,
    offchain_client: &OffChainClient,
    blockchain_retry_tx: mpsc::UnboundedSender<EventWithData>,
    offchain_retry_tx: mpsc::UnboundedSender<QueuedOffChainEvent>,
) {
    debug!(
        "Received event - Channel: {}, Provider: {:?}, ID: {}, Record ID: {}, Timestamp: {}",
        ev.system.channel,
        ev.system.provider.name.clone().unwrap_or("None".to_owned()),
        ev.system.event_id,
        ev.system.event_record_id,
        ev.system.time_created.system_time
    );

    trace!("Event data: {:?}", ev);

    let latest_record = match state_store.get_latest_record(ev.system.channel.as_str()) {
        Ok(v) => v,
        Err(e) => {
            error!("Error reading latest record metadata from state DB: {}", e);
            return;
        }
    };

    let is_new = match latest_record {
        Some(record) => {
            ev.system.event_record_id > record.event_record_id
                || ev.system.time_created.system_time > record.timestamp
        }
        None => true,
    };

    if !is_new {
        debug!(
            "Already received event {} on channel {}, skipping.",
            ev.system.event_record_id, ev.system.channel
        );
        return;
    }

    let metadata = match blockchain_client.submit(ev.clone().into()).await {
        Ok((metadata, hash)) => {
            info!(
                "Stored event {}:{} on blockchain. Event ID: {}",
                ev.system.channel, ev.system.event_record_id, metadata.event_id
            );
            Some((metadata, hash))
        }
        Err(e) => {
            error!(
                "Error storing event {}:{} on blockchain. Adding to the retry list. Error: {}",
                ev.system.channel, ev.system.event_record_id, e
            );

            match blockchain_retry_tx.send(ev.clone()) {
                Ok(_) => info!(
                    "Added event {}:{} to the blockchain retry queue",
                    ev.system.channel, ev.system.event_record_id
                ),
                Err(e) => error!(
                    "Failed to add event {}:{} to the blockchain retry queue: {}",
                    ev.system.channel, ev.system.event_record_id, e
                ),
            }

            None
        }
    };

    // Still update latest record if there was an eror, as we have added the event to the error queue.
    if let Err(e) = state_store.store_latest_record(&ev.clone().into()) {
        error!("Error storing latest record metadata in state DB: {}", e);
        return;
    }

    let (metadata, tx_hash) = match metadata {
        Some(v) => v,
        None => return,
    };

    // Store event off-chain
    match offchain_client
        .submit(metadata.event_id.clone(), tx_hash, ev.event_data.clone())
        .await
    {
        Ok(_) => {
            info!("Stored event {} off-chain successfully.", metadata.event_id);
        }
        Err(e) => {
            error!(
                "Failed to store event {}. Adding to the retry list. Error: {}",
                metadata.event_id, e
            );

            let queued_event =
                QueuedOffChainEvent::new(metadata.event_id.clone(), tx_hash, ev.event_data);
            match offchain_retry_tx.send(queued_event) {
                Ok(_) => info!(
                    "Added event {} to the off-chain retry queue",
                    metadata.event_id
                ),
                Err(e) => error!(
                    "Failed to add event {} to the off-chain retry queue: {}",
                    metadata.event_id, e
                ),
            }
        }
    }
}
