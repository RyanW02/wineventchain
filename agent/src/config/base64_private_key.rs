use base64::prelude::BASE64_STANDARD;
use base64::Engine;
use ed25519_dalek::SigningKey;
use serde::{Deserialize, Serialize, Serializer};
use std::convert::TryInto;

#[derive(Debug, PartialEq, Eq, Clone)]
pub struct Base64PrivateKey(pub SigningKey);

impl Serialize for Base64PrivateKey {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        let encoded = BASE64_STANDARD.encode(self.0.to_keypair_bytes().as_slice());
        serializer.serialize_str(encoded.as_str())
    }
}

impl<'de> Deserialize<'de> for Base64PrivateKey {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        let encoded = String::deserialize(deserializer)?;
        let decoded: [u8; 64] = BASE64_STANDARD
            .decode(encoded.as_bytes())
            .map_err(serde::de::Error::custom)?[..]
            .try_into()
            .map_err(serde::de::Error::custom)?;

        let key = SigningKey::from_keypair_bytes(&decoded).map_err(serde::de::Error::custom)?;
        Ok(Base64PrivateKey(key))
    }
}

impl From<SigningKey> for Base64PrivateKey {
    fn from(val: SigningKey) -> Self {
        Base64PrivateKey(val)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use ed25519_dalek::SigningKey;
    use rand::thread_rng;

    #[derive(Debug, Deserialize, Serialize, PartialEq)]
    struct Wrapper {
        key: Base64PrivateKey,
    }

    #[test]
    fn test_base64_private_key_serde_roundtrip() {
        // Generate seeded signing key
        let signing_key = SigningKey::generate(&mut thread_rng());
        let wrapped = Wrapper {
            key: Base64PrivateKey(signing_key),
        };

        let json = serde_json::to_string(&wrapped)
            .expect("failed to serialize Base64PrivateKey to json string");

        let unwrapped: Wrapper = serde_json::from_str(&json)
            .expect("failed to deserialize Base64PrivateKey from json string");

        assert_eq!(unwrapped, wrapped);
    }
}
