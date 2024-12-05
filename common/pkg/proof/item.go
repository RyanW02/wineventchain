package proof

import "github.com/cometbft/cometbft/proto/tendermint/crypto"

type ItemWithProof[T any] struct {
	Item    *T
	Index   int64
	Height  int64
	ProofOp crypto.ProofOp
}

func (i *ItemWithProof[T]) ProofOps() *crypto.ProofOps {
	return ProofOps(i.ProofOp)
}
