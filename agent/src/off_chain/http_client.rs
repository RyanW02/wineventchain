use std::sync::Arc;

use crate::{
    common::Tester,
    model::{
        events::EventData,
        rpc::{
            offchain::{
                get::GetEventResponse, status::StatusResponse, submit::SubmitRequest,
                ErrorResponse, StoredEvent,
            },
            HexBytes,
        },
    },
    AgentError, Config, Result,
};
use async_trait::async_trait;
use reqwest::StatusCode;
use tendermint::Hash;
use url::Url;

#[derive(Clone, Debug)]
pub struct HttpClient {
    config: Arc<Config>,
    base_url: Url,
    client: reqwest::Client,
}

impl HttpClient {
    #[allow(dead_code)]
    pub fn new(config: Arc<Config>, base_url: Url) -> Self {
        Self::new_with_client(config, base_url, reqwest::Client::new())
    }

    pub fn new_with_client(
        config: Arc<Config>,
        mut base_url: Url,
        client: reqwest::Client,
    ) -> Self {
        base_url.set_path("/");

        Self {
            config,
            base_url,
            client,
        }
    }

    pub async fn status(&self) -> Result<()> {
        let url = self.base_url.join("status")?;
        let response = self.client.get(url).send().await?;

        if response.status().is_success() {
            Ok(())
        } else {
            let status_code = response.status();
            let status: StatusResponse = response.json().await?;
            Err(AgentError::OffChainInterfaceStatusError(
                status_code,
                status.error,
            ))
        }
    }

    pub async fn store_event(
        &self,
        event_id: HexBytes,
        tx_hash: Hash,
        event_data: EventData,
    ) -> Result<()> {
        let data = SubmitRequest::new(event_id, tx_hash, event_data, &self.config.principal);
        let url = self.base_url.join("event")?;

        let response = self.client.post(url).json(&data).send().await?;
        if response.status().is_success() {
            Ok(())
        } else {
            let status_code = response.status();
            let status: ErrorResponse = response.json().await?;

            Err(AgentError::OffChainInterfaceResponseError(
                status_code,
                status.error,
            ))
        }
    }

    pub async fn get_event_data(&self, event_id: HexBytes) -> Result<Option<StoredEvent>> {
        let url = self.base_url.join(&format!("event/{}", event_id))?;
        let response = self.client.get(url).send().await?;

        if response.status().is_success() {
            let data: GetEventResponse = response.json().await?;
            Ok(Some(data.event))
        } else if response.status() == StatusCode::NOT_FOUND {
            Ok(None)
        } else {
            let status_code = response.status();
            let status: ErrorResponse = response.json().await?;

            Err(AgentError::OffChainInterfaceResponseError(
                status_code,
                status.error,
            ))
        }
    }
}

pub struct HttpClientTester;

#[async_trait]
impl Tester for HttpClientTester {
    type Item = HttpClient;

    async fn test(&self, client: &HttpClient) -> bool {
        client.status().await.is_ok()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_url_join() {
        let mut url = Url::parse("http://localhost:8080").unwrap();
        url.set_path("/");

        assert_eq!(
            url.join("status").unwrap().as_str(),
            "http://localhost:8080/status"
        );
    }

    #[test]
    fn test_url_join_with_trailing_slash() {
        let mut url = Url::parse("http://localhost:8080/").unwrap();
        url.set_path("/");

        assert_eq!(
            url.join("status").unwrap().as_str(),
            "http://localhost:8080/status"
        );
    }
}
