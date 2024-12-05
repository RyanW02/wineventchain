package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/RyanW02/wineventchain/chain-client/prompt"
	"github.com/RyanW02/wineventchain/chain-client/validate"
	"github.com/RyanW02/wineventchain/common/pkg/proof"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/common/pkg/types/rpc"
	"go.uber.org/zap"
)

func (c *Client) HandleViewEvent() error {
	eventId, err := prompt.Text("Event ID", validate.Sha256Hash)
	if err != nil {
		return err
	}

	data := rpc.MuxedRequest{App: events.AppName}
	marshalled, err := json.Marshal(data)
	if err != nil {
		return err
	}

	res, err := c.Client.ABCIQueryWithOptions(context.Background(), "/event-by-id/"+eventId, marshalled, ABCIQueryOptions)
	if err != nil {
		return err
	}

	if res.Response.Codespace == events.Codespace && res.Response.Code == events.CodeOk {
		// Validate proof
		if err := proof.ValidateProofOps(res.Response.ProofOps); err != nil {
			return err
		}

		if err := prompt.Display("Event", string(res.Response.Value)); err != nil {
			return err
		}
	} else if res.Response.Codespace == events.Codespace && res.Response.Code == events.CodeEventNotFound {
		if err := prompt.Display("Event", "Event not found"); err != nil {
			return err
		}
	} else if res.Response.Codespace == events.Codespace && res.Response.Code == events.CodeTreeUninitialized {
		if err := prompt.Display("Missing Proof",
			"The blockchain node failed to generate a proof: if the event app is un-seeded, this is normal: "+
				"simply wait for the first event to be submitted and try again. Otherwise, the blockchain node may be "+
				"acting maliciously.",
		); err != nil {
			return err
		}
	} else {
		c.Logger.Warn(
			"Blockchain node returned an error while fetching the event",
			zap.String("codespace", res.Response.Codespace),
			zap.Uint32("code", res.Response.Code),
			zap.String("message", res.Response.Log),
		)

		msg := fmt.Sprintf("Blockchain node returned an error while fetching the event. Code: %s:%d, log: %s", res.Response.Codespace, res.Response.Code, res.Response.Log)
		if err := prompt.Display("Error", msg); err != nil {
			return err
		}
	}

	return c.OpenEventsActionSelector()
}
