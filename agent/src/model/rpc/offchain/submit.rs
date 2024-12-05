use ed25519_dalek::Signer;
use serde::{Deserialize, Serialize};
use tendermint::Hash;

use crate::{
    config::Principal,
    model::{events::EventData, identity, rpc::HexBytes},
};

#[derive(Debug, Deserialize, Serialize)]
pub struct SubmitRequest {
    pub event_id: HexBytes,
    pub tx_hash: HexBytes,
    pub event_data: EventData,
    pub principal: identity::Principal,
    pub signature: String,
}

impl SubmitRequest {
    pub fn new(
        event_id: HexBytes,
        tx_hash: Hash,
        event_data: EventData,
        principal: &Principal,
    ) -> Self {
        let data_hash = event_data.hash();
        let signature = principal.private_key.0.sign(&data_hash);

        Self {
            event_id,
            tx_hash: HexBytes::new(Vec::from(tx_hash.as_bytes())),
            event_data,
            principal: principal.id.clone(),
            signature: hex::encode(signature.to_bytes()),
        }
    }
}
