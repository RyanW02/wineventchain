package multiplexer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cometbft/cometbft/abci/types"
)

type MultiplexedApp interface {
	Name() string
	Info(ctx context.Context, req *types.RequestInfo) any
	InitChain(ctx context.Context, req *types.RequestInitChain) []byte
	CheckTx(ctx context.Context, req *types.RequestCheckTx, data json.RawMessage) (*types.ResponseCheckTx, error)
	FinalizeBlock(ctx context.Context, req *types.RequestFinalizeBlock, data json.RawMessage) FinalizeBlockResponse
	Query(ctx context.Context, req *types.RequestQuery) (*types.ResponseQuery, error)
}

type FinalizeBlockResponse struct {
	TxResult   types.ExecTxResult
	AppHash    []byte
	CommitFunc func() error
}

type ErrorResponse struct {
	Code      uint32 `json:"code"`
	Codespace string `json:"codespace"`
	Log       string `json:"log"`
}

func NewErrorResponse(code uint32, codespace string, err error) *ErrorResponse {
	var log string
	if err != nil {
		log = err.Error()
	}

	return &ErrorResponse{
		Code:      code,
		Codespace: codespace,
		Log:       log,
	}
}

func (e ErrorResponse) Error() string {
	return fmt.Sprintf("[%s %d] %s", e.Codespace, e.Code, e.Log)
}

func (e ErrorResponse) IntoFinalizeBlockResponse() FinalizeBlockResponse {
	txResult := types.ExecTxResult{
		Code:      e.Code,
		Codespace: e.Codespace,
	}

	if e.Log != "" {
		txResult.Log = e.Log
	}

	return FinalizeBlockResponse{
		TxResult:   txResult,
		AppHash:    nil,
		CommitFunc: nil,
	}
}

func (e ErrorResponse) IntoCheckTxResponse() *types.ResponseCheckTx {
	return &types.ResponseCheckTx{
		Code: e.Code,
		Log:  e.Log,
	}
}

func (e ErrorResponse) IntoQueryResponse() *types.ResponseQuery {
	return &types.ResponseQuery{
		Code:      e.Code,
		Log:       e.Log,
		Codespace: e.Codespace,
	}
}
