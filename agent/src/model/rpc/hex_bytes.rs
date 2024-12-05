use core::fmt::Display;

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct HexBytes(pub Vec<u8>);

impl HexBytes {
    pub fn new(bytes: Vec<u8>) -> Self {
        Self(bytes)
    }

    pub fn as_bytes(&self) -> &[u8] {
        &self.0
    }
}

impl Display for HexBytes {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let encoded = hex::encode(self.0.as_slice());
        write!(f, "{}", encoded)
    }
}

impl From<HexBytes> for Vec<u8> {
    fn from(bytes: HexBytes) -> Vec<u8> {
        bytes.0
    }
}

impl From<Vec<u8>> for HexBytes {
    fn from(bytes: Vec<u8>) -> Self {
        Self(bytes)
    }
}

impl From<HexBytes> for String {
    fn from(bytes: HexBytes) -> String {
        hex::encode(bytes.0.as_slice())
    }
}

impl TryFrom<String> for HexBytes {
    type Error = hex::FromHexError;

    fn try_from(s: String) -> Result<Self, Self::Error> {
        let bytes = hex::decode(s)?;
        Ok(Self(bytes))
    }
}

impl Serialize for HexBytes {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let encoded = hex::encode(self.0.as_slice());
        serializer.serialize_str(encoded.as_str())
    }
}

impl<'de> Deserialize<'de> for HexBytes {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        let encoded = String::deserialize(deserializer)?;
        let bytes = hex::decode(encoded).map_err(serde::de::Error::custom)?;
        Ok(Self(bytes))
    }
}
