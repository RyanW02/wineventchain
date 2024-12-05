use crate::model::events::{Metadata, ScrubbedEvent};
use serde::{Deserialize, Serialize};
use uuid::Uuid;

pub const CODESPACE: &str = "events";

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Code {
    Ok = 0,
    UnknownError = 1,
    InvalidQueryPath = 2,
    EventNotFound = 3,
}

impl TryFrom<u32> for Code {
    type Error = ();

    fn try_from(value: u32) -> Result<Self, Self::Error> {
        match value {
            0 => Ok(Self::Ok),
            1 => Ok(Self::UnknownError),
            2 => Ok(Self::InvalidQueryPath),
            3 => Ok(Self::EventNotFound),
            _ => Err(()),
        }
    }
}

#[derive(Debug, Deserialize, Serialize)]
pub struct CreateRequest {
    pub event: ScrubbedEvent,
    pub nonce: Uuid,
}

impl CreateRequest {
    pub fn new(event: ScrubbedEvent) -> Self {
        Self {
            event,
            nonce: Uuid::new_v4(),
        }
    }
}

#[derive(Clone, Debug, PartialEq, Deserialize, Serialize)]
pub struct CreateResponse {
    pub metadata: Metadata,
}
