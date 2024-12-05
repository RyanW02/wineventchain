package repository

import (
	"context"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
	types "github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	"time"
)

type Repository interface {
	Events() EventRepository
	Challenges() ChallengeRepository
	TestConnection() error
}

type EventRepository interface {
	GetEventById(ctx context.Context, id events.EventHash) (events.StoredEvent, bool, error)
	GetEventsById(ctx context.Context, ids []events.EventHash) ([]events.StoredEvent, error)
	GetEventByTx(ctx context.Context, txHash []byte) (events.StoredEvent, bool, error)
	SearchEvents(ctx context.Context, filters []Filter, limit, page int, first *events.EventHash) ([]events.StoredEvent, error)
	EventCount(ctx context.Context) (int, error)
	Store(ctx context.Context, event events.StoredEvent) error
	DropExpiredEvents(ctx context.Context, policy types.RetentionPolicy) error
}

// ChallengeRepository is used to store challenges for authenticating with the viewer server.
type ChallengeRepository interface {
	AddChallenge(ctx context.Context, principal identity.Principal, challenge []byte) error
	GetAndRemoveChallenge(ctx context.Context, principal identity.Principal, challenge []byte, challengeLifetime time.Duration) (bool, error)
	DropExpiredChallenges(ctx context.Context, challengeLifetime time.Duration) error
}
