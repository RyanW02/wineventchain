package rpc

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
)

type RequestType string

type Payload struct {
	Type RequestType     `json:"type"` // Payload type name
	Data json.RawMessage `json:"data"` // Payload type-specific data
}

type UnsignedPayload struct {
	Payload `json:"payload"`
}

type SignedPayload struct {
	Payload   `json:"payload"`
	Principal identity.Principal `json:"principal"` // Who is making the request
	Signature string             `json:"signature"` // Hex-encoded Ed25519 signature of the data
}

const Codespace = "rpc"

const (
	CodeOk = iota
	CodeUnknownRequestType
	CodeInvalidSignature
)

func wrap(payloadType RequestType, payload any) (Payload, error) {
	marshalled, err := json.Marshal(payload)
	if err != nil {
		return Payload{}, err
	}

	return Payload{
		Type: payloadType,
		Data: marshalled,
	}, nil
}

func unsigned(payload Payload) UnsignedPayload {
	return UnsignedPayload{
		Payload: payload,
	}
}

func sign(payload Payload, signer identity.Principal, signerKey ed25519.PrivateKey) (SignedPayload, error) {
	signature := ed25519.Sign(signerKey, payload.Data)

	return SignedPayload{
		Payload:   payload,
		Principal: signer,
		Signature: hex.EncodeToString(signature),
	}, nil
}

func (p *SignedPayload) ValidateSignature(publicKey ed25519.PublicKey) (bool, error) {
	signature, err := hex.DecodeString(p.Signature)
	if err != nil {
		return false, err
	}

	return ed25519.Verify(publicKey, p.Data, signature), nil
}
