package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/RyanW02/wineventchain/chain-client/prompt"
	"github.com/RyanW02/wineventchain/chain-client/validate"
	"github.com/RyanW02/wineventchain/common/pkg/proof"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
	"github.com/RyanW02/wineventchain/common/pkg/types/rpc"
)

func (c *Client) HandleViewPrincipal() error {
	principal, err := prompt.Text("Principal", validate.LengthBetween(1, 255))
	if err != nil {
		return err
	}

	data := rpc.MuxedRequest{App: identity.AppName}
	marshalled, err := json.Marshal(data)
	if err != nil {
		return err
	}

	res, err := c.Client.ABCIQueryWithOptions(context.Background(), "/"+principal, marshalled, ABCIQueryOptions)
	if err != nil {
		return err
	}

	if res.Response.Code == identity.CodeOk {
		// Validate proof
		if err := proof.ValidateProofOps(res.Response.ProofOps); err != nil {
			return err
		}

		if err := prompt.Display("Principal", string(res.Response.Value)); err != nil {
			return err
		}
	} else if res.Response.Codespace == identity.Codespace && res.Response.Code == identity.CodeNotFound {
		if err := prompt.Display("Principal", "Principal not found"); err != nil {
			return err
		}
	} else if res.Response.Codespace == identity.Codespace && res.Response.Code == identity.CodeTreeUninitialized {
		if err := prompt.Display("Missing Proof",
			"The blockchain node failed to generate a proof: if the identity app is un-seeded, this is normal: "+
				"simply seed the app and try again. Otherwise, the blockchain node may be acting maliciously.",
		); err != nil {
			return err
		}
	} else {
		msg := fmt.Sprintf("Unknown error (code %d) - log: %s, info: %s", res.Response.Code, res.Response.Log, res.Response.Info)
		if err := prompt.Display("Principal", msg); err != nil {
			return err
		}
	}

	return c.OpenIdentityActionSelector()
}
