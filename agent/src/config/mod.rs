mod base64_private_key;

use crate::collector::Channel;
use crate::config::base64_private_key::Base64PrivateKey;
use crate::Result;
use ed25519_dalek::SigningKey;
use serde::{Deserialize, Serialize};
use std::fmt::Display;
use std::path::Path;
use std::time::Duration;
use tokio::fs;
use url::Url;

#[derive(Debug, Deserialize, Serialize)]
pub struct Config {
    pub collector: Collector,
    pub blockchain: Blockchain,
    pub off_chain: OffChain,
    pub principal: Principal,

    pub retrieve_past_events: bool,
}

#[derive(Debug, Deserialize, Serialize)]
pub struct Collector {
    pub channels: Vec<Channel>,
    pub state_db_path: String,
}

#[derive(Debug, Deserialize, Serialize)]
pub struct Blockchain {
    pub nodes: Vec<BlockchainNode>,
    pub verification: BlockchainVerificationOptions,
    pub max_retry_queue_size: Option<usize>,
    pub retry_intervals: Vec<String>,
    pub check_interval: String,
}

#[derive(Debug, Deserialize, Serialize)]
pub struct BlockchainVerificationOptions {
    pub health_check_timeout_seconds: u64,

    // Propagation settings
    pub max_propagation_delay: Duration,
    pub propagation_retry_delay: Duration,

    // Consensus settings
    #[serde(default = "three")]
    pub verification_client_count: usize,
    #[serde(default = "three")]
    pub minimum_verification_client_count: usize,
    #[serde(default)]
    pub allow_self_verification: bool,
    #[serde(default)]
    pub max_resubmits: usize,
}

#[derive(Debug, Deserialize, Serialize)]
pub struct BlockchainNode {
    pub http_endpoint: String,
    pub ws_endpoint: Option<String>,
}

#[derive(Debug, Deserialize, Serialize)]
pub struct OffChain {
    pub nodes: Vec<OffChainNode>,
    pub verification: BlockchainVerificationOptions,
    pub max_retry_queue_size: Option<usize>,
    pub retry_intervals: Vec<String>,
    pub check_interval: String,
}

#[derive(Debug, Deserialize, Serialize)]
pub struct OffChainNode {
    pub http_endpoint: url::Url,
}

#[derive(Debug, Deserialize, Serialize)]
pub struct Principal {
    pub id: String,
    pub private_key: Base64PrivateKey,
}

impl Config {
    pub async fn load(path: impl AsRef<Path> + Display) -> Result<Config> {
        if !fs::try_exists(&path).await? {
            log::warn!("{path} not found: writing default config and exiting");
            let default_config = Config::default();
            fs::write(&path, toml::to_string(&default_config)?).await?;
            log::warn!("Default config written to {path}, exiting");
            std::process::exit(0);
        }

        let file = fs::read_to_string(&path).await?;
        toml::from_str(file.as_str()).map_err(Into::into)
    }
}

impl Default for Config {
    fn default() -> Self {
        Self {
            collector: Collector {
                channels: vec![Channel::Application, Channel::Security, Channel::System],
                state_db_path: "state.db".to_string(),
            },
            blockchain: Blockchain {
                nodes: vec![
                    BlockchainNode {
                        http_endpoint: "http://localhost:26657".to_string(),
                        ws_endpoint: Some("ws://localhost:26657/websocket".to_string()),
                    },
                    BlockchainNode {
                        http_endpoint: "http://localhost:26655".to_string(),
                        ws_endpoint: Some("ws://localhost:26655/websocket".to_string()),
                    },
                    BlockchainNode {
                        http_endpoint: "http://localhost:26653".to_string(),
                        ws_endpoint: Some("ws://localhost:26653/websocket".to_string()),
                    },
                ],
                verification: BlockchainVerificationOptions {
                    health_check_timeout_seconds: 3,
                    max_propagation_delay: Duration::from_secs(20),
                    propagation_retry_delay: Duration::from_secs(2),
                    verification_client_count: 3,
                    minimum_verification_client_count: 1,
                    allow_self_verification: true,
                    max_resubmits: 0,
                },
                max_retry_queue_size: Some(500),
                retry_intervals: vec![
                    "1m".to_string(),
                    "5m".to_string(),
                    "15m".to_string(),
                    "30m".to_string(),
                    "1h".to_string(),
                    "3h".to_string(),
                    "6h".to_string(),
                    "1d".to_string(),
                    "3d".to_string(),
                ],
                check_interval: "1m".to_string(),
            },
            off_chain: OffChain {
                nodes: vec![
                    OffChainNode {
                        http_endpoint: Url::parse("http://localhost:8080").unwrap(),
                    },
                    OffChainNode {
                        http_endpoint: Url::parse("http://localhost:8081").unwrap(),
                    },
                    OffChainNode {
                        http_endpoint: Url::parse("http://localhost:8082").unwrap(),
                    },
                ],
                verification: BlockchainVerificationOptions {
                    health_check_timeout_seconds: 3,
                    max_propagation_delay: Duration::from_secs(20),
                    propagation_retry_delay: Duration::from_secs(2),
                    verification_client_count: 3,
                    minimum_verification_client_count: 1,
                    allow_self_verification: true,
                    max_resubmits: 0,
                },
                max_retry_queue_size: Some(500),
                retry_intervals: vec![
                    "1m".to_string(),
                    "5m".to_string(),
                    "15m".to_string(),
                    "30m".to_string(),
                    "1h".to_string(),
                    "3h".to_string(),
                    "6h".to_string(),
                    "1d".to_string(),
                    "3d".to_string(),
                ],
                check_interval: "1m".to_string(),
            },
            principal: Principal {
                id: "user".to_string(),
                private_key: Base64PrivateKey(SigningKey::from_bytes(&[0; 32])),
            },
            retrieve_past_events: false,
        }
    }
}

// Serde default generators
fn three() -> usize {
    3
}
