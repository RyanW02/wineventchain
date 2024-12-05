package factories

import (
	"encoding/hex"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/google/uuid"
	"math/rand"
	"time"
)

func RandomEvent() events.ScrubbedEvent {
	return events.ScrubbedEvent{
		OffChainHash: hex.EncodeToString(events.EventData{}.Hash()),
		Event: events.Event{
			System: events.System{
				Provider: events.Provider{
					Name: Ptr("Random-Provider"),
					Guid: events.NewGuid(uuid.New()),
				},
				EventId: events.EventId(rand.Intn(10_000)),
				TimeCreated: events.TimeCreated{
					SystemTime: time.Now(),
				},
				EventRecordId: rand.Intn(1_000_000),
				Correlation: events.Correlation{
					ActivityId: events.NewGuid(uuid.New()),
				},
				Execution: events.Execution{
					ProcessId: Ptr(rand.Intn(10_000)),
					ThreadId:  Ptr(rand.Intn(20_000)),
				},
				Channel:  "Security",
				Computer: "computer-name",
			},
		},
	}
}

func Ptr[T any](v T) *T {
	return &v
}
