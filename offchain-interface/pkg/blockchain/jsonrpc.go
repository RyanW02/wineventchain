package blockchain

import coretypes "github.com/cometbft/cometbft/rpc/core/types"

type (
	JsonRpcRequest[T any] struct {
		JsonRPCVersion string `json:"jsonrpc"`
		Method         string `json:"method"`
		Id             int    `json:"id"`
		Params         T
	}

	QueryParams struct {
		Query string `json:"query"`
	}

	JsonRpcResponse[T any] struct {
		JsonRPCVersion string `json:"jsonrpc"`
		Id             int
		Result         T
	}

	EventRpcResponse = JsonRpcResponse[coretypes.ResultEvent]
)

func newSubscribeRequest(id int, query string) JsonRpcRequest[QueryParams] {
	return JsonRpcRequest[QueryParams]{
		JsonRPCVersion: "2.0",
		Method:         "subscribe",
		Id:             id,
		Params: QueryParams{
			Query: query,
		},
	}
}
