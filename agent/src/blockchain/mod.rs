mod retry_queue;
pub use retry_queue::start_retry_loop;

use crate::common::{FailureStrategy, Tester, VerifyingClient, VerifyingClientOptions};
use crate::model::events::{EventWithMetadata, Metadata, ScrubbedEvent};
use crate::model::rpc::blockchain::events::{self, CreateRequest, CreateResponse};
use crate::model::rpc::blockchain::{AppName, MuxedRequest, Payload, QueryData, RequestType};
use crate::model::rpc::HexBytes;
use crate::{AgentError, Config, Result};
use async_trait::async_trait;
use backoff::exponential::ExponentialBackoffBuilder;
use backoff::Error;
use base64::prelude::BASE64_STANDARD;
use base64::Engine;
use log::{error, trace};
use std::sync::Arc;
use std::time::Duration;
use tendermint::Hash;
use tendermint_rpc::endpoint::broadcast::tx_sync::Response;
use tendermint_rpc::{Client, HttpClient};

pub struct BlockchainClient {
    config: Arc<Config>,
    client: VerifyingClient<HttpClient, TendermintTester>,
}

struct TendermintTester;

#[async_trait]
impl Tester for TendermintTester {
    type Item = HttpClient;

    async fn test(&self, client: &HttpClient) -> bool {
        client.health().await.is_ok()
    }
}

impl BlockchainClient {
    pub fn new(config: Arc<Config>) -> Self {
        let rpc_clients = config
            .blockchain
            .nodes
            .iter()
            .map(|node| HttpClient::new(node.http_endpoint.as_str()))
            .collect::<std::result::Result<Vec<_>, _>>()
            .expect("Failed to create HTTP clients");

        let opts = VerifyingClientOptions {
            health_check_timeout: Duration::from_secs(
                config.blockchain.verification.health_check_timeout_seconds,
            ),
            verification_client_count: config.blockchain.verification.verification_client_count,
            minimum_verification_client_count: config
                .blockchain
                .verification
                .minimum_verification_client_count,
            allow_self_verification: config.blockchain.verification.allow_self_verification,
            max_resubmits: config.blockchain.verification.max_resubmits,
            failure_strategy: FailureStrategy::Error,
        };

        let client = VerifyingClient::new(rpc_clients, TendermintTester {}, opts);
        Self { config, client }
    }

    pub async fn health(&self) -> bool {
        self.client.get(&[]).await.is_some()
    }

    pub async fn submit(&self, event: ScrubbedEvent) -> Result<(Metadata, Hash)> {
        let backoff_config = ExponentialBackoffBuilder::new()
            .with_initial_interval(self.config.blockchain.verification.propagation_retry_delay)
            .with_multiplier(1.25)
            .with_max_interval(self.config.blockchain.verification.propagation_retry_delay * 4)
            .with_max_elapsed_time(Some(
                self.config.blockchain.verification.max_propagation_delay,
            ))
            .build();

        let res: Arc<(Response, Metadata)> = self
            .client
            .run(
                backoff_config.clone(),
                |client: HttpClient| {
                    let event = event.clone();
                    let backoff_config = backoff_config.clone();
                    async move {
                        let payload = MuxedRequest {
                            app: AppName::Events,
                            data: Payload {
                                r#type: RequestType::EventCreate,
                                data: CreateRequest::new(event),
                            }
                                .sign(&self.config.principal)?,
                        };

                        let marshalled = serde_json::to_string(&payload)?;
                        trace!("Sending blockchain request: {}", marshalled);

                        let res = client.broadcast_tx_sync(&marshalled[..]).await?;

                        if res.code.is_err() {
                            error!(
                                "Error submitting transaction (code {}): {}",
                                res.code.value(),
                                res.log
                            );
                            return Err(AgentError::BlockchainError(res.code.value(), res.log));
                        }

                        let metadata = backoff::future::retry(backoff_config, || async {
                            trace!("Fetching tx from blockchain");

                            let query_res = match client.tx(res.hash, true).await {
                                Ok(v) => v,
                                Err(e) => {
                                    if e.to_string().contains("not found") {
                                        trace!("Tx was not found");
                                        return Err(Error::transient(AgentError::TendermintError(e)));
                                    } else {
                                        error!("Error fetching submitted create transaction from node: {}", e);
                                        return Err(Error::permanent(AgentError::TendermintError(e)));
                                    }
                                }
                            };

                            trace!("Got tx");

                            if query_res.tx_result.code.is_err() {
                                error!(
                                    "Error submitting fetching submitted create transaction (code {}): {}",
                                    query_res.tx_result.code.value(),
                                    query_res.tx_result.log
                                );

                                return Err(Error::permanent(AgentError::BlockchainError(
                                    query_res.tx_result.code.value(),
                                    query_res.tx_result.log,
                                )));
                            }

                            // tx_result.data is base64 decoded (Golang quirk), so decode it first
                            let decoded = BASE64_STANDARD.decode(&query_res.tx_result.data)
                                .map_err(AgentError::from)
                                .map_err(Error::permanent)?;

                            trace!("Tx result data: {:?}", std::str::from_utf8(&decoded).unwrap_or("invalid utf-8"));

                            serde_json::from_slice::<CreateResponse>(&decoded[..])
                                .map(|res| res.metadata)
                                .map_err(AgentError::from)
                                .map_err(Error::permanent)
                        }).await?;

                        Ok((res, metadata))
                    }
                },
                |client: HttpClient, res: Arc<(Response, Metadata)>| async move {
                    let query_res = match client.tx(res.0.hash, true).await {
                        Ok(v) => v,
                        Err(e) => {
                            if e.to_string().contains("not found") {
                                trace!("Tx was not found");
                                return Err(Error::transient(AgentError::TendermintError(e)));
                            } else {
                                error!("Error fetching submitted create transaction for verification: {}", e);
                                return Err(Error::permanent(AgentError::TendermintError(e)));
                            }
                        }
                    };

                    if query_res.tx_result.code.is_err() {
                        error!(
                            "Error submitting fetching submitted create transaction for verification (code {}): {}",
                            query_res.tx_result.code.value(),
                            query_res.tx_result.log
                        );

                        return Err(Error::permanent(AgentError::BlockchainError(query_res.tx_result.code.value(), query_res.tx_result.log)));
                    }

                    // tx_result.data is base64 decoded (Golang quirk), so decode it first
                    let decoded = match BASE64_STANDARD.decode(&query_res.tx_result.data) {
                        Ok(v) => v,
                        Err(e) => {
                            error!("Error decoding Base64 tx_result.data in event submit verification");
                            return Err(Error::permanent(AgentError::Base64DecodeError(e)));
                        }
                    };

                    trace!("Verification result data: {:?}", std::str::from_utf8(&decoded).unwrap_or("invalid utf-8"));

                    let decoded: CreateResponse = match serde_json::from_slice(&decoded) {
                        Ok(v) => v,
                        Err(e) => {
                            error!("Error deserializing create response for verification: {}", e);
                            return Err(Error::permanent(AgentError::JsonError(e)));
                        }
                    };

                    Ok(decoded.metadata == res.1)
                },
            )
            .await?;

        Ok((res.1.clone(), res.0.hash))
    }

    pub async fn get_event(&self, event_id: HexBytes) -> Result<Option<EventWithMetadata>> {
        let data = QueryData::new(AppName::Events);
        let client = self
            .client
            .get(&[])
            .await
            .ok_or(AgentError::NoClientsAvailable)?;

        let path = format!("/event-by-id/{}", event_id);
        let data_json = serde_json::to_string(&data)?;

        let res = client
            .client
            .abci_query(Some(path), data_json, None, true)
            .await?;

        if res.code.is_err() && res.codespace == events::CODESPACE {
            let code = if let Ok(code) = events::Code::try_from(res.code.value()) {
                code
            } else {
                return Err(AgentError::ABCIError(
                    res.codespace,
                    res.code.value(),
                    res.log,
                ));
            };

            if code == events::Code::EventNotFound {
                return Ok(None);
            } else {
                return Err(AgentError::EventsABCIError(code, res.log));
            }
        }

        let event: EventWithMetadata =
            serde_json::from_slice(&res.value).map_err(AgentError::JsonError)?;

        Ok(Some(event))
    }
}
