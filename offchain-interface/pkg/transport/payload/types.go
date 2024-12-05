package payload

import "github.com/RyanW02/wineventchain/common/pkg/types/events"

// EventRequest is used to request event data from another node. JSON keys are a single character to reduce
// bandwidth usage, which is essential when using multicast.
type EventRequest struct {
	EventIds []events.EventHash `json:"e"`
}

// EventBackfillResponse is a response to an EventRequest. The receiver should request the blockchain to validate
// the hash of the EventData. JSON keys are a single character to reduce bandwidth usage, which is essential when
// using multicast.
type EventBackfillResponse struct {
	Events []EventBackfillResponseData `json:"e"`
}

type EventBackfillResponseData struct {
	EventId   events.EventHash `json:"e"`
	TxHash    events.TxHash    `json:"h"`
	EventData events.EventData `json:"d"`
}

func NewEventRequest(eventIds []events.EventHash) EventRequest {
	return EventRequest{
		EventIds: eventIds,
	}
}

func (r *EventBackfillResponse) EventIds() []events.EventHash {
	ids := make([]events.EventHash, len(r.Events))
	for i, event := range r.Events {
		ids[i] = event.EventId
	}
	return ids
}
