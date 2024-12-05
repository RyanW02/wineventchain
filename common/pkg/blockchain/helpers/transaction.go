package helpers

import (
	"context"
	"errors"
	"fmt"
	"github.com/cometbft/cometbft/rpc/client/http"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	rpctypes "github.com/cometbft/cometbft/rpc/jsonrpc/types"
	"github.com/cometbft/cometbft/types"
	"strings"
	"time"
)

func BroadcastAndPollDefault(ctx context.Context, client *http.HTTP, tx types.Tx) (*coretypes.ResultTx, error) {
	return BroadcastAndPoll(ctx, client, tx, 200*time.Millisecond, 15*time.Second)
}

func BroadcastAndPoll(ctx context.Context, client *http.HTTP, tx types.Tx, retryFrequency time.Duration, timeout time.Duration) (*coretypes.ResultTx, error) {
	res, err := client.BroadcastTxSync(ctx, tx)
	if err != nil {
		return nil, err
	}

	if res.Code != 0 {
		return nil, fmt.Errorf("Transaction failed! Code %s:%d; Message: %s", res.Codespace, res.Code, res.Log)
	}

	ctx, cancelFunc := context.WithTimeout(ctx, timeout)
	defer cancelFunc()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryFrequency):
			res, err := client.Tx(ctx, res.Hash, true)
			if err != nil {
				var rpcError *rpctypes.RPCError
				if errors.As(err, &rpcError) && strings.Contains(rpcError.Data, "not found") {
					continue
				}
				return nil, err
			}

			return res, nil
		}
	}
}
