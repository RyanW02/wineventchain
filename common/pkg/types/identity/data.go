package identity

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
)

type IdentityData struct {
	PublicKey ed25519.PublicKey `json:"public_key"`
	Role      Role              `json:"role"`
}

func (i IdentityData) IsAdmin() bool {
	return i.Role == RoleAdmin
}

// MarshalJSON Custom marshaller to encode public key as hex string
func (i IdentityData) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		PublicKey string `json:"public_key"`
		Role      Role   `json:"role"`
	}{
		PublicKey: hex.EncodeToString(i.PublicKey),
		Role:      i.Role,
	})
}

// UnmarshalJSON Custom unmarshaller to decode public key from hex string
func (i *IdentityData) UnmarshalJSON(data []byte) error {
	var aux struct {
		PublicKey string `json:"public_key"`
		Role      Role   `json:"role"`
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	publicKey, err := hex.DecodeString(aux.PublicKey)
	if err != nil {
		return err
	}

	i.PublicKey = publicKey
	i.Role = aux.Role

	return nil
}
