package events

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"testing"
)

func TestMarshalTxHashJSON(t *testing.T) {
	type wrapper struct {
		TxHash TxHash `json:"tx_hash"`
	}

	w := wrapper{TxHash{0x01, 0x02, 0x03, 0x04}}

	marshalled, err := json.Marshal(w)
	require.NoError(t, err)

	require.Equal(t, `{"tx_hash":"01020304"}`, string(marshalled))
}

func TestUnmarshalTxHashJSON(t *testing.T) {
	type wrapper struct {
		TxHash TxHash `json:"tx_hash"`
	}

	var w wrapper
	err := json.Unmarshal([]byte(`{"tx_hash":"01020304"}`), &w)
	require.NoError(t, err)

	require.Equal(t, TxHash{0x01, 0x02, 0x03, 0x04}, w.TxHash)
}

func TestMarshalUnmarshalTxHashBSON(t *testing.T) {
	type wrapper struct {
		TxHash TxHash `bson:"tx_hash"`
	}

	w := wrapper{TxHash{0x01, 0x02, 0x03, 0x04}}

	marshalled, err := bson.Marshal(w)
	require.NoError(t, err)

	var w2 wrapper
	err = bson.Unmarshal(marshalled, &w2)
	require.NoError(t, err)

	require.Equal(t, w, w2)
}
