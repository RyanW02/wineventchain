package identity

import (
	"errors"
	"github.com/RyanW02/wineventchain/common/pkg/proof"
	types "github.com/RyanW02/wineventchain/common/pkg/types/identity"
	"github.com/cometbft/cometbft/libs/json"
	"github.com/cosmos/iavl"
	"sync"
)

type MerkleRepository struct {
	tree *iavl.MutableTree
	mu   sync.Mutex
}

const (
	metaPrefix      = "meta/"
	principalPrefix = "principal/"
)

var _ Repository = (*MerkleRepository)(nil)

var ErrNotFound = errors.New("identity not found")

func NewMerkleRepository(tree *iavl.MutableTree) *MerkleRepository {
	return &MerkleRepository{
		tree: tree,
		mu:   sync.Mutex{},
	}
}

func (r *MerkleRepository) LoadLatest() (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.tree.Load()
}

func (r *MerkleRepository) LoadVersion(version int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.tree.LoadVersion(version)
	return err
}

func (r *MerkleRepository) Rollback() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tree.Rollback()
}

func (r *MerkleRepository) Hash() ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.tree.Hash()
}

func (r *MerkleRepository) Save() ([]byte, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.tree.SaveVersion()
}

func (r *MerkleRepository) Get(principal types.Principal) (types.IdentityData, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	bytes, err := r.tree.Get([]byte(principalPrefix + principal))
	if err != nil {
		return types.IdentityData{}, err
	}

	if bytes == nil {
		return types.IdentityData{}, ErrNotFound
	}

	var data types.IdentityData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return types.IdentityData{}, err
	}

	return data, nil
}

func (r *MerkleRepository) GetWithProof(principal types.Principal) (proof.ItemWithProof[types.IdentityData], error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := []byte(principalPrefix + principal)
	index, bytes, err := r.tree.GetWithIndex(key)
	if err != nil {
		return proof.ItemWithProof[types.IdentityData]{}, err
	}

	// Generate merkle proof
	proofOp, err := proof.ProofOpForTree(r.tree, key)
	if err != nil {
		return proof.ItemWithProof[types.IdentityData]{}, err
	}

	// Key does not exist, but we still need to return a proof
	if bytes == nil {
		return proof.ItemWithProof[types.IdentityData]{
			Item:    nil,
			Index:   index,
			Height:  proof.GetProofHeight(r.tree),
			ProofOp: proofOp,
		}, nil
	}

	var data types.IdentityData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return proof.ItemWithProof[types.IdentityData]{}, err
	}

	return proof.ItemWithProof[types.IdentityData]{
		Item:    &data,
		Index:   index,
		Height:  proof.GetProofHeight(r.tree),
		ProofOp: proofOp,
	}, nil
}

func (r *MerkleRepository) Has(principal types.Principal) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.tree.Has([]byte(principalPrefix + principal))
}

func (r *MerkleRepository) Store(principal types.Principal, data types.IdentityData) error {
	marshalled, err := json.Marshal(data)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	_, err = r.tree.Set([]byte(principalPrefix+principal), marshalled)
	return err
}

func (r *MerkleRepository) IsSeeded() (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.tree.Has([]byte(metaPrefix + "seeded"))
}

func (r *MerkleRepository) SetSeeded() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.tree.Set([]byte(metaPrefix+"seeded"), []byte("true"))
	return err
}
