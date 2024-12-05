package state

import (
	"context"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"time"
)

type Store[T any, U any] interface {
	Close(ctx context.Context) error
	LastSeenBlockHeight(ctx context.Context) (*int64, error)
	SetLastSeenBlockHeight(ctx context.Context, height int64) error
	MissingBlocks(ctx context.Context) ([]BlockRangeWithId[T], error)
	AddMissingBlocks(ctx context.Context, blockRange BlockRange) (T, error)
	UpdateMissingBlockRange(ctx context.Context, blockRange BlockRangeWithId[T]) error
	RemoveMissingBlockRange(ctx context.Context, id T) error
	MissingEvents(ctx context.Context, after *U, limit int) ([]MissingEventWithKey[U], error)
	MissingEventCount(ctx context.Context) (int, error)
	AddMissingEvents(ctx context.Context, event ...MissingEvent) ([]MissingEventWithKey[U], error)
	IncrementMissingEventRetryCount(ctx context.Context, eventId events.EventHash) error
	RemoveMissingEvent(ctx context.Context, eventId events.EventHash) error
}

type BlockRange struct {
	Low  int64 // Inclusive
	High int64 // Exclusive
}

type BlockRangeWithId[T any] struct {
	BlockRange
	Id T
}

func NewBlockRange(low, high int64) BlockRange {
	return BlockRange{
		Low:  low,
		High: high,
	}
}

func NewBlockRangeWithId[T any](blockRange BlockRange, id T) BlockRangeWithId[T] {
	return BlockRangeWithId[T]{
		BlockRange: blockRange,
		Id:         id,
	}
}

type MissingEvent struct {
	// Event data
	EventId      events.EventHash
	ReceivedTime time.Time
	BlockHeight  int64

	// Retry data
	LastRetryTime  time.Time
	RetriedUnicast bool
	RetryCount     int
}

type MissingEventWithKey[U any] struct {
	MissingEvent
	Id U
}

func NewMissingEvent(eventId events.EventHash, receivedTime time.Time, blockHeight int64) MissingEvent {
	return MissingEvent{
		EventId:        eventId,
		ReceivedTime:   receivedTime,
		BlockHeight:    blockHeight,
		RetriedUnicast: false,
		RetryCount:     0,
	}
}
