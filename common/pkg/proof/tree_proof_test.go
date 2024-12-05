package proof

import (
	dbm "github.com/cometbft/cometbft-db"
	"github.com/cometbft/cometbft/proto/tendermint/crypto"
	"github.com/cosmos/iavl"
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func TestValidatePresent(t *testing.T) {
	tree := generateTree(t, 100)
	proof := generateProof(t, tree, []byte("key17"))

	valid, err := Validate(proof)
	require.NoErrorf(t, err, "error validating proof")
	require.Truef(t, valid, "proof verification failed")
}

func TestValidatePresentAll(t *testing.T) {
	treeSize := 500
	tree := generateTree(t, treeSize)

	for i := 0; i < treeSize; i++ {
		proof := generateProof(t, tree, []byte("key"+strconv.Itoa(i)))

		valid, err := Validate(proof)
		require.NoErrorf(t, err, "error validating proof for key %d", i)
		require.Truef(t, valid, "proof verification failed for key %d", i)
	}
}

func TestValidateAbsent(t *testing.T) {
	tree := generateTree(t, 100)
	proof := generateProof(t, tree, []byte("does_not_exist"))

	valid, err := Validate(proof)
	require.NoErrorf(t, err, "error validating proof")
	require.Truef(t, valid, "proof verification failed")
}

func TestValidatePresentSerialisation(t *testing.T) {
	tree := generateTree(t, 100)
	proof := generateProof(t, tree, []byte("key98"))

	serialised, err := proof.Marshal()
	require.NoErrorf(t, err, "error serialising proof")

	deserialised := crypto.ProofOp{}
	err = deserialised.Unmarshal(serialised)
	require.NoErrorf(t, err, "error deserialising proof")

	valid, err := Validate(deserialised)
	require.NoErrorf(t, err, "error validating proof")
	require.Truef(t, valid, "proof verification failed")
}

func TestValidateAbsentSerialisation(t *testing.T) {
	tree := generateTree(t, 100)
	proof := generateProof(t, tree, []byte("does_not_exist"))

	serialised, err := proof.Marshal()
	require.NoErrorf(t, err, "error serialising proof")

	deserialised := crypto.ProofOp{}
	err = deserialised.Unmarshal(serialised)
	require.NoErrorf(t, err, "error deserialising proof")

	valid, err := Validate(deserialised)
	require.NoErrorf(t, err, "error validating proof")
	require.Truef(t, valid, "proof verification failed")
}

func generateProof(t *testing.T, tree *iavl.MutableTree, key []byte) crypto.ProofOp {
	op, err := ProofOpForTree(tree, key)
	require.NoErrorf(t, err, "error generating proof op")

	return op
}

func generateTree(t *testing.T, size int) *iavl.MutableTree {
	db := dbm.NewMemDB()
	tree, err := iavl.NewMutableTree(db, 100, true)
	require.NoErrorf(t, err, "error creating tree")

	for i := 0; i < size; i++ {
		key := []byte("key" + strconv.Itoa(i))
		_, err := tree.Set(key, []byte("value"))
		require.NoErrorf(t, err, "error setting key")
	}

	_, _, err = tree.SaveVersion()
	require.NoErrorf(t, err, "error saving tree")

	return tree
}
