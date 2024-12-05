package payload

import "encoding/json"

type (
	PayloadType uint8

	Payload struct {
		Type PayloadType     `json:"t"`
		Data json.RawMessage `json:"d"`
	}
)

const (
	TypeBroadcastEvent   PayloadType = iota // Received event from agent, broadcasting data
	TypeRequestEvent                        // Who has this event?
	TypeBackfillResponse                    // Here is the event data
)

func NewPayload(t PayloadType, data any) (*Payload, error) {
	rawData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return &Payload{
		Type: t,
		Data: rawData,
	}, nil
}

func NewPayloadMarshalled(t PayloadType, data any) ([]byte, error) {
	p, err := NewPayload(t, data)
	if err != nil {
		return nil, err
	}

	return json.Marshal(p)
}
