use crate::{model::events::Event, Result};
use serde::{Deserialize, Serialize};
use time::OffsetDateTime;

use super::{StateStore, KEY_PREFIX};

#[derive(Debug, Deserialize, Serialize)]
pub struct LatestRecord {
    pub event_record_id: usize,
    pub timestamp: OffsetDateTime,
}

impl StateStore {
    pub fn get_latest_record(&self, channel: &str) -> Result<Option<LatestRecord>> {
        let res = self.db.get(self.latest_record_key(channel))?;
        match res {
            Some(v) => {
                let record: LatestRecord = serde_json::from_slice(&v[..])?;
                Ok(Some(record))
            }
            None => Ok(None),
        }
    }

    pub fn store_latest_record(&self, event: &Event) -> Result<()> {
        let latest_record = LatestRecord {
            event_record_id: event.system.event_record_id,
            timestamp: event.system.time_created.system_time,
        };

        let serialized = serde_json::to_vec(&latest_record)?;

        self.db.insert(
            self.latest_record_key(event.system.channel.as_str()),
            serialized,
        )?;

        Ok(())
    }

    fn latest_record_key(&self, channel: &str) -> String {
        format!("{}_latest_record_{}", KEY_PREFIX, channel)
    }
}
