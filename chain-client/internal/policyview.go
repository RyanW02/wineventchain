package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/RyanW02/wineventchain/chain-client/prompt"
	"github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	"github.com/RyanW02/wineventchain/common/pkg/types/retention"
	"github.com/RyanW02/wineventchain/common/pkg/types/rpc"
	"go.uber.org/zap"
)

func (c *Client) HandlePolicyView() error {
	data := rpc.MuxedRequest{App: retention.AppName}
	marshalled, err := json.Marshal(data)
	if err != nil {
		return err
	}

	res, err := c.Client.ABCIQueryWithOptions(context.Background(), "/", marshalled, ABCIQueryOptionsNoProve)
	if err != nil {
		return err
	}

	if res.Response.Codespace == retention.Codespace && res.Response.Code == retention.CodeOk {
		var policy offchain.StoredPolicy
		if err := json.Unmarshal(res.Response.Value, &policy); err != nil {
			return err
		}

		if err := prompt.DisplayMarshalled("Policy", policy); err != nil {
			return err
		}
	} else if res.Response.Codespace == retention.Codespace && res.Response.Code == retention.CodePolicyNotSet {
		if err := prompt.Display("Policy", "Policy not set"); err != nil {
			return err
		}
	} else {
		c.Logger.Warn(
			"Blockchain node returned an error while fetching the retention policy",
			zap.String("codespace", res.Response.Codespace),
			zap.Uint32("code", res.Response.Code),
			zap.String("message", res.Response.Log),
		)

		msg := fmt.Sprintf(
			"Blockchain node returned an error while fetching the retention policy. Code: %s:%d, log: %s",
			res.Response.Codespace,
			res.Response.Code,
			res.Response.Log,
		)

		if err := prompt.Display("Error", msg); err != nil {
			return err
		}
	}

	return c.OpenRetentionPolicyActionSelector()
}
