package events

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

type TxHash []byte

// Enforce compile-time interface conformance
var (
	_ fmt.Stringer          = (*TxHash)(nil)
	_ json.Marshaler        = (*TxHash)(nil)
	_ json.Unmarshaler      = (*TxHash)(nil)
	_ bson.ValueMarshaler   = (*TxHash)(nil)
	_ bson.ValueUnmarshaler = (*TxHash)(nil)
)

func (t TxHash) String() string {
	return hex.EncodeToString(t)
}

func (t TxHash) MarshalJSON() ([]byte, error) {
	return []byte(`"` + hex.EncodeToString(t) + `"`), nil
}

func (t *TxHash) UnmarshalJSON(data []byte) error {
	if len(data) < 2 {
		return errors.New("invalid TxHash - length less than 2")
	}

	decoded, err := hex.DecodeString(string(data[1 : len(data)-1]))
	if err != nil {
		return err
	}

	*t = decoded
	return nil
}

func (t TxHash) MarshalBSONValue() (bsontype.Type, []byte, error) {
	return bson.MarshalValue(hex.EncodeToString(t))
}

func (t *TxHash) UnmarshalBSONValue(b bsontype.Type, bytes []byte) error {
	var s string
	if err := bson.UnmarshalValue(b, bytes, &s); err != nil {
		return err
	}

	decoded, err := hex.DecodeString(s)
	if err != nil {
		return err
	}

	*t = decoded
	return nil
}
