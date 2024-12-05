mod deserializer;
mod guid;

pub use guid::Guid;

use crate::model::identity::Principal;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use time::OffsetDateTime;

use super::rpc::HexBytes;

#[derive(Clone, Debug, PartialEq, Eq, Deserialize, Serialize)]
pub struct ScrubbedEvent {
    // SHA256 hash of the event data.
    pub offchain_hash: String,
    pub event: Event,
}

#[derive(Clone, Debug, PartialEq, Eq, Deserialize, Serialize)]
pub struct EventWithMetadata {
    #[serde(flatten)]
    pub event: ScrubbedEvent,
    pub metadata: Metadata,
}

#[derive(Clone, Debug, PartialEq, Eq, Deserialize, Serialize)]
pub struct EventWithData {
    pub system: System,
    pub event_data: EventData,
}

#[derive(Clone, Debug, PartialEq, Eq, Deserialize, Serialize)]
pub struct Event {
    pub system: System,
}

#[derive(Clone, Debug, PartialEq, Eq, Deserialize, Serialize)]
pub struct Metadata {
    // The unique identifier of the event.
    pub event_id: HexBytes,
    // The time when the event was received by the blockchain node.
    #[serde(with = "time::serde::rfc3339")]
    pub received_time: OffsetDateTime,
    // The identity of the entity that generated the event.
    pub principal: Principal,
}

#[derive(Clone, Debug, PartialEq, Eq, Deserialize, Serialize)]
pub struct System {
    // Information about the source that generated the event.
    pub provider: Provider,
    // The numeric ID of the event *type*.
    pub event_id: EventId,
    // Timestamps associated to the creation of the event.
    pub time_created: TimeCreated,
    // A (local) unique identifier for the event.
    pub event_record_id: usize,
    // Information that can be used to correlate multiple related events.
    pub correlation: Correlation,
    // Information about the process that generated the event.
    pub execution: Execution,
    // The name of the event log channel that the event was received on, e.g. Security, System, etc.
    pub channel: String,
    // The local hostname of the machine that generated the event.
    pub computer: String,
}

// The numeric ID of the event type.
pub type EventId = usize;

#[derive(Clone, Debug, PartialEq, Eq, Deserialize, Serialize)]
pub struct Provider {
    // The name of the source that generated the event.
    pub name: Option<String>,
    // The globally unique identifier of the event source. It is a string of the form {UUID}.
    pub guid: Option<Guid>,
    // An alternative name of the event source that generated the event.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub event_source_name: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Eq, Deserialize, Serialize)]
pub struct TimeCreated {
    // The local time of the computer that generated the event, at the timestamp the event was
    // generated locally.
    #[serde(with = "time::serde::rfc3339")]
    pub system_time: OffsetDateTime,
}

#[derive(Clone, Debug, PartialEq, Eq, Deserialize, Serialize)]
pub struct Correlation {
    // A Guid assigned to multiple events that are part of the same activity that can be used to
    // correlate multiple related events. It is a string of the form {UUID}.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub activity_id: Option<Guid>,
}

#[derive(Clone, Debug, PartialEq, Eq, Deserialize, Serialize)]
pub struct Execution {
    // The numeric ID of the process that generated the event.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub process_id: Option<usize>,
    // The numeric ID of the OS thread that the event was generated on.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub thread_id: Option<usize>,
}

#[derive(Clone, Debug, PartialEq, Eq, Deserialize, Serialize)]
pub struct EventData(Vec<Data>);

#[derive(Clone, Debug, PartialEq, Eq, Deserialize, Serialize)]
pub struct Data {
    pub name: Option<String>,
    pub value: Option<String>,
}

impl From<EventWithData> for Event {
    fn from(val: EventWithData) -> Self {
        Self { system: val.system }
    }
}

impl From<EventWithData> for ScrubbedEvent {
    fn from(val: EventWithData) -> Self {
        Self {
            offchain_hash: hex::encode(val.event_data.hash()),
            event: val.into(),
        }
    }
}

impl EventData {
    pub fn hash(&self) -> [u8; 32] {
        let mut digest = Sha256::new();
        self.0.iter().for_each(|data| {
            if let Some(name) = &data.name {
                Digest::update(&mut digest, name.as_bytes());
            }

            if let Some(value) = &data.value {
                Digest::update(&mut digest, value.as_bytes());
            }
        });

        digest.finalize().into()
    }
}
