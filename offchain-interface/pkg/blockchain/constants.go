package blockchain

import "github.com/cometbft/cometbft/rpc/client"

var ABCIQueryOptions = client.ABCIQueryOptions{
	Height: 0,
	Prove:  true,
}
