mod muxed_request;

pub use muxed_request::{AppName, MuxedRequest, QueryData};

pub mod events;

use ed25519_dalek::{Signature, Signer, Verifier};

use crate::config::Principal;
use crate::model::identity;
use crate::Result;
use serde::{Deserialize, Serialize};
use serde_json::value::{to_raw_value, RawValue};

#[allow(dead_code)]
#[derive(Debug, Deserialize, Serialize)]
pub enum RequestType {
    // Identity app requests
    #[serde(rename = "seed")]
    IdentitySeed,
    #[serde(rename = "register")]
    IdentityRegister,

    // Events app requests
    #[serde(rename = "create")]
    EventCreate,
}

#[derive(Debug, Deserialize, Serialize)]
pub struct Payload<T: Serialize> {
    pub r#type: RequestType,
    pub data: T,
}

impl<T: Serialize> Payload<T> {
    pub fn new(r#type: RequestType, data: T) -> Self {
        Self { r#type, data }
    }

    pub fn sign(self, principal: &Principal) -> Result<SignedPayload> {
        let raw_value = to_raw_value(&self)?;
        let serialized = serde_json::to_vec(&self.data)?;
        let signature = principal.private_key.0.sign(&serialized[..]);

        Ok(SignedPayload {
            payload: raw_value,
            principal: principal.id.clone(),
            signature: hex::encode(signature.to_bytes()),
        })
    }
}

#[derive(Debug, Deserialize, Serialize)]
pub struct SignedPayload {
    pub payload: Box<RawValue>,
    pub principal: identity::Principal,
    pub signature: String,
}

impl SignedPayload {
    #[allow(dead_code)]
    pub fn verify(&self, principal: Principal) -> Result<bool> {
        let signature = Signature::from_slice(hex::decode(&self.signature)?.as_slice())?;
        let public_key = principal.private_key.0.verifying_key();
        Ok(public_key
            .verify(self.payload.get().as_bytes(), &signature)
            .is_ok())
    }
}
