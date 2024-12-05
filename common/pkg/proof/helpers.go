package proof

import "github.com/cometbft/cometbft/proto/tendermint/crypto"

func ProofOps(proofOp crypto.ProofOp) *crypto.ProofOps {
	return &crypto.ProofOps{Ops: []crypto.ProofOp{proofOp}}
}
