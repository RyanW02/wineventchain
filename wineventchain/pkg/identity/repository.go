package identity

import (
	"github.com/RyanW02/wineventchain/app/internal/datastore"
	"github.com/RyanW02/wineventchain/common/pkg/proof"
	types "github.com/RyanW02/wineventchain/common/pkg/types/identity"
)

type Repository interface {
	datastore.BaseRepository
	Get(principal types.Principal) (types.IdentityData, error)
	GetWithProof(principal types.Principal) (proof.ItemWithProof[types.IdentityData], error)
	Has(principal types.Principal) (bool, error)
	Store(principal types.Principal, data types.IdentityData) error
	IsSeeded() (bool, error)
	SetSeeded() error
}
