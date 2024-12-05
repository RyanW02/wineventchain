use serde::{Deserialize, Serialize};
use windows::core::{w, PCWSTR};

use crate::AgentError;

#[derive(Clone, Copy, Debug, Eq, Hash, PartialEq, Deserialize, Serialize)]
pub enum Channel {
    #[serde(alias = "application")]
    Application,
    #[serde(alias = "security")]
    Security,
    #[serde(alias = "setup")]
    Setup,
    #[serde(alias = "system")]
    System,
}

impl Channel {
    pub fn as_pcwstr(&self) -> PCWSTR {
        match self {
            Channel::Application => w!("Application"),
            Channel::Security => w!("Security"),
            Channel::Setup => w!("Setup"),
            Channel::System => w!("System"),
        }
    }
}

impl From<Channel> for PCWSTR {
    fn from(value: Channel) -> Self {
        value.as_pcwstr()
    }
}

impl TryFrom<String> for Channel {
    type Error = AgentError;

    fn try_from(value: String) -> Result<Self, Self::Error> {
        match value.to_lowercase().as_str() {
            "application" => Ok(Self::Application),
            "security" => Ok(Self::Security),
            "setup" => Ok(Self::Setup),
            "system" => Ok(Self::System),
            _ => AgentError::InvalidVariant(value).into(),
        }
    }
}
