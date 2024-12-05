use std::str::FromStr;

use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::AgentError;

#[derive(Clone, Copy, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]
pub struct Guid(pub Uuid);

impl Guid {
    pub fn new() -> Self {
        Self(Uuid::new_v4())
    }

    pub fn parse(s: &str) -> crate::Result<Self> {
        Ok(Self(Uuid::parse_str(s)?))
    }
}

impl Default for Guid {
    fn default() -> Self {
        Self::new()
    }
}

impl FromStr for Guid {
    type Err = AgentError;

    fn from_str(s: &str) -> crate::Result<Self> {
        Guid::parse(s).map_err(Into::into)
    }
}

impl Serialize for Guid {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        let mut s = String::new();
        s.push('{');
        s.push_str(&self.0.to_string());
        s.push('}');

        serializer.serialize_str(s.as_str())
    }
}

impl<'de> Deserialize<'de> for Guid {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        let s = String::deserialize(deserializer)?;
        let stripped = s.trim_start_matches('{').trim_end_matches('}');
        let parsed = Uuid::parse_str(stripped).map_err(serde::de::Error::custom)?;

        Ok(Self(parsed))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::str::FromStr;

    #[test]
    fn test_guid_serialize() {
        let guid = Guid::new();
        let serialized = serde_json::to_string(&guid).unwrap();
        assert_eq!(serialized.len(), 40);

        // { and } are escaped as {{ and }}
        assert_eq!(serialized, format!("\"{{{}}}\"", guid.0.to_string()));
    }

    #[test]
    fn test_guid_deserialize() {
        let s = "\"{dad0c443-b88a-44c2-a9b1-ce0b4bdc96f2}\"";
        let deserialized: Guid = serde_json::from_str(s).unwrap();

        let uuid = Uuid::from_str("dad0c443-b88a-44c2-a9b1-ce0b4bdc96f2").unwrap();
        assert_eq!(Guid(uuid), deserialized);
    }

    #[test]
    fn test_guid_deserialize_round_trip() {
        let guid = Guid::new();
        let serialized = serde_json::to_string(&guid).unwrap();
        let deserialized: Guid = serde_json::from_str(&serialized).unwrap();
        assert_eq!(guid, deserialized);
    }
}
