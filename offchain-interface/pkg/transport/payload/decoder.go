package payload

import (
	"encoding/json"
	"fmt"
	types "github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	"go.uber.org/zap"
	"sync"
)

type (
	Decoder struct {
		logger                       *zap.Logger
		mu                           sync.RWMutex
		broadcastHandler             BroadcastHandler
		eventRequestHandler          EventRequestHandler
		eventBackfillResponseHandler EventBackfillResponseHandler
	}

	BroadcastHandler             func(logger *zap.Logger, sourceName string, request types.SubmitRequest)
	EventRequestHandler          func(logger *zap.Logger, sourceName string, request EventRequest)
	EventBackfillResponseHandler func(logger *zap.Logger, sourceName string, response EventBackfillResponse)
)

func NewDecoder(logger *zap.Logger) *Decoder {
	return &Decoder{
		logger: logger,
	}
}

func (d *Decoder) WithBroadcastHandler(handler BroadcastHandler) *Decoder {
	d.SetBroadcastHandler(handler)
	return d
}

func (d *Decoder) SetBroadcastHandler(handler BroadcastHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.broadcastHandler = handler
}

func (d *Decoder) WithRequestHandler(handler EventRequestHandler) *Decoder {
	d.SetRequestHandler(handler)
	return d
}

func (d *Decoder) SetRequestHandler(handler EventRequestHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.eventRequestHandler = handler
}

func (d *Decoder) WithEventBackfillResponseHandler(handler EventBackfillResponseHandler) *Decoder {
	d.SetEventBackfillResponseHandler(handler)
	return d
}

func (d *Decoder) SetEventBackfillResponseHandler(handler EventBackfillResponseHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.eventBackfillResponseHandler = handler
}

func (d *Decoder) HandleMessage(sourceName string, message []byte) error {
	payload, err := decodePayload(message)
	if err != nil {
		return err
	}

	switch payload.Type {
	case TypeBroadcastEvent:
		d.mu.RLock()
		defer d.mu.RUnlock()

		var data types.SubmitRequest
		if err := json.Unmarshal(payload.Data, &data); err != nil {
			return err
		}

		if d.broadcastHandler != nil {
			go d.broadcastHandler(d.logger, sourceName, data)
		}
	case TypeRequestEvent:
		d.mu.RLock()
		defer d.mu.RUnlock()

		var data EventRequest
		if err := json.Unmarshal(payload.Data, &data); err != nil {
			return err
		}

		if d.eventRequestHandler != nil {
			go d.eventRequestHandler(d.logger, sourceName, data)
		}
	case TypeBackfillResponse:
		d.mu.RLock()
		defer d.mu.RUnlock()

		var data EventBackfillResponse
		if err := json.Unmarshal(payload.Data, &data); err != nil {
			return err
		}

		if d.eventBackfillResponseHandler != nil {
			go d.eventBackfillResponseHandler(d.logger, sourceName, data)
		}
	default:
		return fmt.Errorf("unknown payload type %d", payload.Type)
	}

	return nil
}

func decodePayload(bytes []byte) (*Payload, error) {
	var p Payload
	if err := json.Unmarshal(bytes, &p); err != nil {
		return nil, err
	}

	return &p, nil
}
