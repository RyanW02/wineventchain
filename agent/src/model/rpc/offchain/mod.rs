pub mod get;
pub mod status;
pub mod submit;

use serde::{Deserialize, Serialize};

use crate::{
    blockchain::BlockchainClient,
    model::events::{EventWithData, Metadata},
    AgentError, Result,
};

use super::{Base64Bytes, HexBytes};

#[derive(Debug, Deserialize, Serialize)]
pub struct ErrorResponse {
    pub error: Option<String>,
}

#[derive(Debug, Deserialize, Serialize)]
pub struct StoredEvent {
    pub event: EventWithData,
    pub metadata: Metadata,
    pub tx_hash: Base64Bytes,
}

impl StoredEvent {
    pub fn new(event: EventWithData, metadata: Metadata, tx_hash: Base64Bytes) -> Self {
        Self {
            event,
            metadata,
            tx_hash,
        }
    }

    pub async fn validate(
        &self,
        event_id: HexBytes,
        blockchain_client: &BlockchainClient,
    ) -> Result<()> {
        let hash = hex::encode(self.event.event_data.hash());
        let on_chain = blockchain_client
            .get_event(event_id.clone())
            .await?
            .ok_or_else(|| AgentError::BlockchainEventNotFound(event_id))?;

        if hash != on_chain.event.offchain_hash {
            return Err(AgentError::OffChainHashValidationFailed(
                on_chain.event.offchain_hash,
                hash,
            ));
        }

        Ok(())
    }
}
