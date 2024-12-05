use serde::{Deserialize, Serialize};

use super::StoredEvent;

#[derive(Debug, Deserialize, Serialize)]
pub struct GetEventResponse {
    pub event: StoredEvent,
}
