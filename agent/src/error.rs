use thiserror::Error;

use crate::model::rpc::{blockchain::events::Code, HexBytes};

pub type Result<T> = std::result::Result<T, AgentError>;

#[derive(Debug, Error)]
pub enum AgentError {
    #[error("IO error: {0}")]
    IoError(#[from] std::io::Error),

    #[error("TOML deserialization error: {0}")]
    TomlDeError(#[from] toml::de::Error),

    #[error("TOML serialization error: {0}")]
    TomlSerError(#[from] toml::ser::Error),

    #[error("JSON error: {0}")]
    JsonError(#[from] serde_json::Error),

    #[error("Signature error: {0}")]
    SignatureError(#[from] ed25519_dalek::SignatureError),

    #[error("Error decoding hex value: {0}")]
    HexError(#[from] hex::FromHexError),

    #[error("No clients available in VerifyingClient")]
    NoClientsAvailable,

    #[error("Not enough clients available in VerifyingClient: {0} < {1}")]
    NotEnoughClientsAvailable(usize, usize),

    #[error("Verification failed")]
    VerificationFailed,

    #[error("Blockchain operation returned error (code {0}): {1}")]
    BlockchainError(u32, String),

    #[error("Tendermint error: {0}")]
    TendermintError(#[from] tendermint_rpc::Error),

    #[error("Join error: {0}")]
    JoinError(#[from] tokio::task::JoinError),

    #[error("Error operating on Win32 API: {0}")]
    Win32Error(#[from] windows::core::Error),

    #[error("Error parsing XML: {0}")]
    XMLParseError(#[from] roxmltree::Error),

    #[error("Error parsing integer: {0}")]
    ParseIntError(#[from] std::num::ParseIntError),

    #[error("Error parsing UUID: {0}")]
    UuidError(#[from] uuid::Error),

    #[error("Error parsing time: {0}")]
    TimeParseError(#[from] time::error::Parse),

    #[error("Missing attribute when performing deserialization: {0}")]
    MissingAttribute(String),

    #[error("Error rendering event (status: {0})")]
    EvtRenderError(u32),

    #[error("Error operating on Sled database: {0}")]
    SledError(#[from] sled::Error),

    #[error("Encountered unknown variant when performing conversion: {0}")]
    InvalidVariant(String),

    #[error("Error decoding Base64 data: {0}")]
    Base64DecodeError(#[from] base64::DecodeError),

    #[error("Error performing HTTP request: {0}")]
    ReqwestError(#[from] reqwest::Error),

    #[error("Error parsing URL: {0}")]
    UrlParseError(#[from] url::ParseError),

    #[error("OffChain Interface health check returned error: status code {0}, error: {1:?}")]
    OffChainInterfaceStatusError(reqwest::StatusCode, Option<String>),

    #[error("OffChain Interface operation returned an error: status code {0}, error: {1:?}")]
    OffChainInterfaceResponseError(reqwest::StatusCode, Option<String>),

    #[error("Error performing ABCI operation: {0}:{1} - {2}")]
    ABCIError(String, u32, String),

    #[error("Error performing ABCI operation on Events app: {0:?} - {1}")]
    EventsABCIError(Code, String),

    #[error("Event not found on blockchain: {0}")]
    BlockchainEventNotFound(HexBytes),

    #[error("Error validating off-chain hash: expected {0}, got {1}")]
    OffChainHashValidationFailed(String, String),

    #[error("Error converting bytes to UTF-8 string: {0}")]
    Utf8Error(#[from] std::str::Utf8Error),

    #[error("Retry queue has already been started")]
    RetryQueueAlreadyStarted,

    #[error("Error parsing duration: {0}")]
    DurationParseError(#[from] duration_str::DError),

    #[error("Error parsing integer: {0}")]
    TryFromIntError(#[from] std::num::TryFromIntError),

    #[error("Other: {0}")]
    Other(String),
}

impl<T> From<AgentError> for Result<T> {
    fn from(val: AgentError) -> Self {
        Err(val)
    }
}
