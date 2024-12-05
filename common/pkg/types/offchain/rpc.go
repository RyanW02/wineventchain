package offchain

import (
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
)

type (
	SubmitRequest struct {
		// EventId is a unique identifier for the event.
		EventId events.EventHash `json:"event_id"`
		// TxHash is the hash of the transaction that contains the event.
		TxHash events.TxHash `json:"tx_hash"`
		// EventData is the additional data of the event to be stored off-chain.
		EventData events.EventData `json:"event_data"`
		// Principal is the identity of the entity that generated the event, and is submitting this request.
		Principal string `json:"principal"`
		// Signature is a signed SHA256 hash of the event data.
		Signature string `json:"signature"`
	}

	StorageStatus string

	GetEventResponse struct {
		Event events.StoredEvent `json:"event,omitempty"`
	}
)
