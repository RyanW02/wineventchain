use crate::model::rpc::blockchain::SignedPayload;
use serde::{Deserialize, Serialize};
use std::fmt::Debug;

#[derive(Debug, Deserialize, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum AppName {
    Identity,
    Events,
}

#[derive(Debug, Deserialize, Serialize)]
pub struct MuxedRequest {
    pub app: AppName,
    pub data: SignedPayload,
}

impl MuxedRequest {
    pub fn new(app: AppName, data: SignedPayload) -> Self {
        Self { app, data }
    }
}

#[derive(Debug, Deserialize, Serialize)]
pub struct QueryData {
    pub app: AppName,
}

impl QueryData {
    pub fn new(app: AppName) -> Self {
        Self { app }
    }
}
