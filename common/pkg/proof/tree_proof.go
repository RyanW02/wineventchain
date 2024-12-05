package proof

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cometbft/cometbft/proto/tendermint/crypto"
	ics23 "github.com/confio/ics23/go"
	"github.com/cosmos/iavl"
)

type TreeProof struct {
	AppHash []byte                 `json:"app_hash"`
	Proof   *ics23.CommitmentProof `json:"proof"`
}

const TypeIAVL = "ics23:iavl"

var (
	ErrMissingProof      = errors.New("missing proof")
	ErrInvalidProof      = errors.New("invalid proof")
	ErrTreeUninitialized = errors.New("uninitialized merkle tree: cannot generate proof for empty tree")
)

func ProofOpForTree(tree *iavl.MutableTree, key []byte) (crypto.ProofOp, error) {
	hash, err := tree.Hash()
	if err != nil {
		return crypto.ProofOp{}, err
	}

	proof, err := tree.GetProof(key)
	if err != nil {
		// No strict error type available
		if err.Error() == "cannot generate the proof with nil root" {
			return crypto.ProofOp{}, ErrTreeUninitialized
		} else {
			return crypto.ProofOp{}, err
		}
	}

	treeProof := TreeProof{
		AppHash: hash,
		Proof:   proof,
	}

	marshalled, err := json.Marshal(treeProof)
	if err != nil {
		return crypto.ProofOp{}, err
	}

	return crypto.ProofOp{
		Type: TypeIAVL,
		Key:  key,
		Data: marshalled,
	}, nil
}

func Validate(proof crypto.ProofOp) (bool, error) {
	var treeProof TreeProof
	if err := json.Unmarshal(proof.Data, &treeProof); err != nil {
		return false, err
	}

	if membershipProof, ok := treeProof.Proof.Proof.(*ics23.CommitmentProof_Exist); ok {
		return membershipProof.Exist.Verify(
			ics23.IavlSpec,
			treeProof.AppHash,
			proof.Key,
			membershipProof.Exist.Value,
		) == nil, nil
	} else if nonMembershipProof, ok := treeProof.Proof.Proof.(*ics23.CommitmentProof_Nonexist); ok {
		return nonMembershipProof.Nonexist.Verify(ics23.IavlSpec, treeProof.AppHash, proof.Key) == nil, nil
	} else {
		return false, fmt.Errorf("unsupported proof type %T", treeProof.Proof.Proof)
	}
}

func ValidateProofOps(proofOps *crypto.ProofOps) error {
	if proofOps == nil || len(proofOps.Ops) == 0 {
		return ErrMissingProof
	}

	valid, err := Validate(proofOps.Ops[0])
	if err != nil {
		return err
	}

	if !valid {
		return ErrInvalidProof
	}

	return nil
}

func (t TreeProof) MarshalJSON() ([]byte, error) {
	proofMarshalled, err := t.Proof.Marshal()
	if err != nil {
		return nil, err
	}

	return json.Marshal(struct {
		AppHash []byte `json:"app_hash"`
		Proof   []byte `json:"proof"`
	}{
		AppHash: t.AppHash,
		Proof:   proofMarshalled,
	})
}

func (t *TreeProof) UnmarshalJSON(b []byte) error {
	var raw struct {
		AppHash []byte `json:"app_hash"`
		Proof   []byte `json:"proof"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	proof := &ics23.CommitmentProof{}
	if err := proof.Unmarshal(raw.Proof); err != nil {
		return err
	}

	t.AppHash = raw.AppHash
	t.Proof = proof
	return nil
}
