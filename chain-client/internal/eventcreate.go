package internal

import (
	"context"
	"encoding/hex"
	"encoding/xml"
	"github.com/RyanW02/wineventchain/chain-client/prompt"
	"github.com/RyanW02/wineventchain/chain-client/validate"
	"github.com/RyanW02/wineventchain/common/pkg/blockchain/helpers"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
	"github.com/RyanW02/wineventchain/common/pkg/types/rpc"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"math/rand"
	"os"
	"time"
)

func (c *Client) HandleCreateEvent() error {
	return prompt.SelectAndExecute("Choose an option",
		prompt.NewSelectOption("Import From XML File", "📥", c.HandleImportEvent),
		prompt.NewSelectOption("Generate Random Event", "🎲", c.HandleGenerateEvent),
		prompt.NewSelectOption("Back", "⬅️", c.OpenEventsActionSelector),
	)
}

func (c *Client) HandleImportEvent() error {
	filePath, err := prompt.Text("Path to XML file", validate.FileExists)
	if err != nil {
		return err
	}

	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var event events.EventWithData
	if err := xml.Unmarshal(bytes, &event); err != nil {
		return err
	}

	return c.submitEvent(events.ScrubbedEvent{
		OffChainHash: hex.EncodeToString(event.EventData.Hash()),
		Event:        event.Event,
	})
}

func (c *Client) HandleGenerateEvent() error {
	event := events.ScrubbedEvent{
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

	return c.submitEvent(event)
}

func (c *Client) submitEvent(event events.ScrubbedEvent) error {
	if c.ActivePrincipal == nil || c.ActivePrivateKey == nil {
		if err := prompt.Display("Error", "No active principal"); err != nil {
			return err
		}

		return c.OpenMainMenu()
	}

	marshalled, err := rpc.NewBuilder().
		App(events.AppName).
		Data(events.RequestTypeCreate, events.CreateRequest{
			Event: event,
			Nonce: uuid.New(),
		}).
		Signed(identity.Principal(*c.ActivePrincipal), c.ActivePrivateKey).
		Marshal()

	if err != nil {
		return err
	}

	// Submit transaction
	res, err := helpers.BroadcastAndPollDefault(context.Background(), c.Client, marshalled)
	if err != nil {
		return err
	}

	// Check if the transaction was successful
	if res.TxResult.Code == events.CodeOk {
		c.Logger.Info("Event logged successfully")
		c.Logger.Info(res.TxResult.Log)
		c.Logger.Info(res.TxResult.Info)

		if err := prompt.Display("Event Metadata", string(res.TxResult.Data)); err != nil {
			return err
		}
	} else {
		if len(res.TxResult.Log) == 0 {
			c.Logger.Error("failed to store event", zap.Uint32("code", res.TxResult.Code))
			return c.HandleCreateEvent()
		} else {
			c.Logger.Error(
				"failed to store event",
				zap.Uint32("code", res.TxResult.Code),
				zap.String("log", res.TxResult.Log),
			)
			return c.HandleCreateEvent()
		}
	}

	return c.OpenEventsActionSelector()
}
