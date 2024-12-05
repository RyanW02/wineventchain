package rpc

import "encoding/json"

type MuxedRequest struct {
	App  string          `json:"app"`
	Data json.RawMessage `json:"data"`
}
