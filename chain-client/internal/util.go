package internal

import (
	"crypto/ed25519"
	"encoding/base64"
	"github.com/cometbft/cometbft/rpc/client"
	"os"
)

var ABCIQueryOptions = client.ABCIQueryOptions{
	Height: 0,
	Prove:  true,
}

var ABCIQueryOptionsNoProve = client.ABCIQueryOptions{
	Height: 0,
	Prove:  false,
}

func LoadPrivateKey(keyFile string) (ed25519.PrivateKey, error) {
	bytes, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}

	key := make([]byte, base64.StdEncoding.DecodedLen(len(bytes)))
	n, err := base64.StdEncoding.Decode(key, bytes)
	if err != nil {
		return nil, err
	}

	return key[:n], nil
}

func WritePrivateKey(keyFile string, key ed25519.PrivateKey) error {
	bytes := make([]byte, base64.StdEncoding.EncodedLen(len(key)))
	base64.StdEncoding.Encode(bytes, key)

	return os.WriteFile(keyFile, bytes, 0600)
}

func Ptr[T any](t T) *T {
	return &t
}
