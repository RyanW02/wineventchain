package identity

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
)

const (
	AppName = "identity"

	// RequestTypeSeed is used to seed the identity service with an admin user
	RequestTypeSeed = "seed"
	// RequestTypeRegister is used to register a new user
	RequestTypeRegister = "register"
)

type PayloadSeed struct {
	Principal Principal         `json:"principal"` // The principal to register as the admin user
	Key       ed25519.PublicKey // The Ed25519 public key of the principal. Hex encoded in transit.
}

// MarshalJSON Custom Marshaller to encode public key as hex string
func (p PayloadSeed) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Principal Principal `json:"principal"`
		Key       string    `json:"key"`
	}{
		Principal: p.Principal,
		Key:       hex.EncodeToString(p.Key),
	})
}

// UnmarshalJSON Custom unmarshaller to decode public key from hex string
func (p *PayloadSeed) UnmarshalJSON(data []byte) error {
	var aux struct {
		Principal Principal `json:"principal"`
		Key       string    `json:"key"`
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	key, err := hex.DecodeString(aux.Key)
	if err != nil {
		return err
	}

	p.Principal = aux.Principal
	p.Key = key

	return nil
}

type PayloadRegister struct {
	Principal Principal         `json:"principal"`  // The principal to register
	Role      Role              `json:"role"`       // The role to assign to the principal
	Key       ed25519.PublicKey `json:"public_key"` // The Ed25519 public key of the principal. Hex encoded in transit.
}

// MarshalJSON Custom Marshaller to encode public key as hex string
func (p PayloadRegister) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Principal Principal `json:"principal"`
		Role      Role      `json:"role"`
		Key       string    `json:"public_key"`
		Nonce     string    `json:"nonce"` // Prevents CometBFT's "tx already exists in cache" error.
	}{
		Principal: p.Principal,
		Role:      p.Role,
		Key:       hex.EncodeToString(p.Key),
		Nonce:     uuid.New().String(),
	})
}

// UnmarshalJSON Custom unmarshaller to decode public key from hex string
func (p *PayloadRegister) UnmarshalJSON(data []byte) error {
	var aux struct {
		Principal Principal `json:"principal"`
		Role      Role      `json:"role"`
		Key       string    `json:"public_key"`
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	key, err := hex.DecodeString(aux.Key)
	if err != nil {
		return err
	}

	p.Principal = aux.Principal
	p.Role = aux.Role
	p.Key = key

	return nil
}
