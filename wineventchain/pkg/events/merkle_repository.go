package events

import (
	"encoding/json"
	"github.com/RyanW02/wineventchain/common/pkg/proof"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/cometbft/cometbft/libs/sync"
	"github.com/cosmos/iavl"
	"github.com/pkg/errors"
)

type MerkleRepository struct {
	tree *iavl.MutableTree
	mu   sync.Mutex
}

var _ Repository = (*MerkleRepository)(nil)

var (
	ErrNotFound = errors.New("event not found")
)

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

func (r *MerkleRepository) GetByEventId(id events.EventHash) (events.EventWithMetadata, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	bytes, err := r.tree.Get(id)
	if err != nil {
		return events.EventWithMetadata{}, err
	}

	if bytes == nil {
		return events.EventWithMetadata{}, ErrNotFound
	}

	var event events.EventWithMetadata
	if err := json.Unmarshal(bytes, &event); err != nil {
		return events.EventWithMetadata{}, err
	}

	return event, nil
}

func (r *MerkleRepository) GetWithProof(id events.EventHash) (proof.ItemWithProof[events.EventWithMetadata], error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	index, bytes, err := r.tree.GetWithIndex(id)
	if err != nil {
		return proof.ItemWithProof[events.EventWithMetadata]{}, err
	}

	// Generate merkle proof
	proofOp, err := proof.ProofOpForTree(r.tree, id)
	if err != nil {
		return proof.ItemWithProof[events.EventWithMetadata]{}, err
	}

	// Item not found
	if bytes == nil {
		return proof.ItemWithProof[events.EventWithMetadata]{
			Item:    nil,
			Index:   index,
			Height:  proof.GetProofHeight(r.tree),
			ProofOp: proofOp,
		}, nil
	}

	var event events.EventWithMetadata
	if err := json.Unmarshal(bytes, &event); err != nil {
		return proof.ItemWithProof[events.EventWithMetadata]{}, err
	}

	return proof.ItemWithProof[events.EventWithMetadata]{
		Item:    &event,
		Index:   index,
		Height:  proof.GetProofHeight(r.tree),
		ProofOp: proofOp,
	}, nil
}

func (r *MerkleRepository) EventCount() (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	size := r.tree.Size()
	if size < 0 {
		return 0, errors.New("negative size")
	} else {
		return uint64(size), nil
	}
}

func (r *MerkleRepository) Store(event events.EventWithMetadata) error {
	marshalled, err := json.Marshal(event)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	_, err = r.tree.Set(event.Metadata.EventId, marshalled)
	return err
}
