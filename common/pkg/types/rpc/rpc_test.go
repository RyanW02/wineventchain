package rpc

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSignatureValidation(t *testing.T) {
	userPub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoErrorf(t, err, "failed to generate key pair: %v", err)

	adminPub, adminPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoErrorf(t, err, "failed to generate key pair: %v", err)

	payloadRegister := identity.PayloadRegister{
		Principal: "new_user",
		Role:      identity.RoleUser,
		Key:       userPub,
	}

	wrapped, err := wrap(identity.RequestTypeRegister, payloadRegister)
	require.NoErrorf(t, err, "failed to wrap payload: %v", err)

	payload, err := sign(wrapped, "admin", adminPriv)
	require.NoErrorf(t, err, "failed to create signed rpc: %v", err)

	payloadJson, err := json.Marshal(payload)
	require.NoErrorf(t, err, "failed to marshal rpc: %v", err)

	var payloadParsed SignedPayload
	require.NoErrorf(t, json.Unmarshal(payloadJson, &payloadParsed), "failed to unmarshal rpc: %v", err)

	valid, err := payloadParsed.ValidateSignature(adminPub)
	require.NoErrorf(t, err, "failed to validate signature: %v", err)
	require.Truef(t, valid, "signature validation failed")

	require.Equal(t, payload, payloadParsed)
}

func TestSignatureArbitraryPayload(t *testing.T) {
	adminPub, adminPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoErrorf(t, err, "failed to generate key pair: %v", err)

	payload := SignedPayload{
		Payload: Payload{
			Type: "abc",
			Data: []byte(`"def"`),
		},
		Principal: "admin",
		Signature: hex.EncodeToString(ed25519.Sign(adminPriv, []byte(`"def"`))),
	}

	payloadJson, err := json.Marshal(payload)
	require.NoErrorf(t, err, "failed to marshal rpc: %v", err)

	// Decode
	var payloadParsed SignedPayload
	require.NoErrorf(t, json.Unmarshal(payloadJson, &payloadParsed), "failed to unmarshal rpc: %v", err)

	// Validate
	valid, err := payloadParsed.ValidateSignature(adminPub)
	require.NoErrorf(t, err, "failed to validate signature: %v", err)
	require.Truef(t, valid, "signature validation failed")

	require.Equal(t, payload, payloadParsed)
}

func TestInvalidSignature(t *testing.T) {
	userPub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoErrorf(t, err, "failed to generate key pair: %v", err)

	adminPub, adminPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoErrorf(t, err, "failed to generate key pair: %v", err)

	payloadRegister := identity.PayloadRegister{
		Principal: "new_user",
		Role:      identity.RoleUser,
		Key:       userPub,
	}

	wrapped, err := wrap(identity.RequestTypeRegister, payloadRegister)
	require.NoErrorf(t, err, "failed to wrap payload: %v", err)

	payload, err := sign(wrapped, "admin", adminPriv)
	require.NoErrorf(t, err, "failed to create signed rpc: %v", err)

	payload.Signature = hex.EncodeToString([]byte("different data"))

	payloadJson, err := json.Marshal(payload)
	require.NoErrorf(t, err, "failed to marshal rpc: %v", err)

	var payloadParsed SignedPayload
	require.NoErrorf(t, json.Unmarshal(payloadJson, &payloadParsed), "failed to unmarshal rpc: %v", err)

	valid, err := payloadParsed.ValidateSignature(adminPub)
	require.NoErrorf(t, err, "failed to validate signature: %v", err)
	require.Falsef(t, valid, "signature validation succeeded when it should have failed")
}
