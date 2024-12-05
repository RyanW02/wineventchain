use std::sync::Arc;

mod latest_record;

pub struct StateStore {
    db: Arc<sled::Db>,
}

const KEY_PREFIX: &str = "state";

impl StateStore {
    pub fn new(db: Arc<sled::Db>) -> Self {
        Self { db }
    }
}
