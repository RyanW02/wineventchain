use base64::{prelude::BASE64_STANDARD, Engine};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Base64Bytes(pub Vec<u8>);

impl Base64Bytes {
    pub fn new(bytes: Vec<u8>) -> Self {
        Self(bytes)
    }

    pub fn as_bytes(&self) -> &[u8] {
        &self.0
    }
}

impl From<Base64Bytes> for Vec<u8> {
    fn from(bytes: Base64Bytes) -> Vec<u8> {
        bytes.0
    }
}

impl From<Vec<u8>> for Base64Bytes {
    fn from(bytes: Vec<u8>) -> Self {
        Self(bytes)
    }
}

impl From<Base64Bytes> for String {
    fn from(bytes: Base64Bytes) -> String {
        BASE64_STANDARD.encode(bytes.0)
    }
}

impl TryFrom<String> for Base64Bytes {
    type Error = base64::DecodeError;

    fn try_from(s: String) -> Result<Self, Self::Error> {
        let bytes = BASE64_STANDARD.decode(s)?;
        Ok(Self(bytes))
    }
}

impl Serialize for Base64Bytes {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let encoded = BASE64_STANDARD.encode(self.0.as_slice());
        serializer.serialize_str(encoded.as_str())
    }
}

impl<'de> Deserialize<'de> for Base64Bytes {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        let encoded = String::deserialize(deserializer)?;
        let bytes = BASE64_STANDARD
            .decode(encoded)
            .map_err(serde::de::Error::custom)?;
        Ok(Self(bytes))
    }
}
