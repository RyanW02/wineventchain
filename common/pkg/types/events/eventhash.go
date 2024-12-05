package events

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
)

// EventHash is a hex-encoded SHA-256 hash of the event metadata (System), along with the block height and principal.
type EventHash = TxHash

func NewEventHash(blockHeight uint64, principal identity.Principal, event Event) (EventHash, error) {
	digest := sha256.New()

	heightBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(heightBytes, blockHeight)
	if _, err := digest.Write(heightBytes); err != nil {
		return nil, err
	}

	if _, err := digest.Write([]byte(principal)); err != nil {
		return nil, err
	}

	// Marshal the event to JSON bytes to hash. JSON encoding in Go is deterministic, as the keys are sorted first.
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	if _, err := digest.Write(eventBytes); err != nil {
		return nil, err
	}

	return digest.Sum(nil), nil
}
