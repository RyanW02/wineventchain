package events

import (
	"github.com/RyanW02/wineventchain/app/internal/datastore"
	"github.com/RyanW02/wineventchain/common/pkg/proof"
	types "github.com/RyanW02/wineventchain/common/pkg/types/events"
)

type Repository interface {
	datastore.BaseRepository
	GetByEventId(id types.EventHash) (types.EventWithMetadata, error)
	GetWithProof(id types.EventHash) (proof.ItemWithProof[types.EventWithMetadata], error)
	EventCount() (uint64, error)
	Store(event types.EventWithMetadata) error
}
