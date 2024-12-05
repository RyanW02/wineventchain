package events

import "github.com/google/uuid"

const (
	AppName = "events"

	// RequestTypeCreate is used to create a new event
	RequestTypeCreate = "create"
)

type CreateRequest struct {
	Event ScrubbedEvent `json:"event"`
	// Nonce is a unique identifier for the request. Prevents "tx already exists in cache" errors from Tendermint.
	Nonce uuid.UUID `json:"nonce"`
}

type CreateResponse struct {
	Metadata Metadata `json:"metadata"`
}

type EventCountResponse struct {
	Count uint64 `json:"count"`
}
