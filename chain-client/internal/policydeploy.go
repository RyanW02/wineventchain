package internal

import (
	"context"
	"github.com/RyanW02/wineventchain/chain-client/prompt"
	"github.com/RyanW02/wineventchain/chain-client/validate"
	"github.com/RyanW02/wineventchain/common/pkg/blockchain/helpers"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
	"github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	"github.com/RyanW02/wineventchain/common/pkg/types/retention"
	"github.com/RyanW02/wineventchain/common/pkg/types/rpc"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"os"
)

func (c *Client) HandlePolicyDeploy() error {
	filePath, err := prompt.Text("Path to YAML policy file", validate.FileExists)
	if err != nil {
		return err
	}

	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var policy offchain.RetentionPolicy
	if err := yaml.Unmarshal(bytes, &policy); err != nil {
		return err
	}

	if err := policy.Validate(); err != nil {
		if err := prompt.Display("Policy Validation Error", "Policy is invalid: "+err.Error()); err != nil {
			return err
		}

		return c.OpenRetentionPolicyActionSelector()
	}

	marshalled, err := rpc.NewBuilder().
		App(retention.AppName).
		Data(retention.RequestTypeSetPolicy, retention.SetPolicyRequest{
			Policy: policy,
			Nonce:  uuid.New(),
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
		c.Logger.Info("Policy set successfully")

		if err := prompt.Display("Policy Set", "Retention policy set successfully!"); err != nil {
			return err
		}
	} else {
		if res.TxResult.Codespace == retention.Codespace && res.TxResult.Code == retention.CodePolicyAlreadySet {
			if err := prompt.Display("Policy Already Set", "A retention policy has already been set"); err != nil {
				return err
			}

			return c.OpenRetentionPolicyActionSelector()
		} else if res.TxResult.Codespace == retention.Codespace && res.TxResult.Code == retention.CodeUnauthorized {
			if err := prompt.Display("Unauthorized", "You are not authorized to set the retention policy: the admin role is required to be assigned to the principal making the request"); err != nil {
				return err
			}

			return c.OpenRetentionPolicyActionSelector()
		} else {
			if len(res.TxResult.Log) == 0 {
				c.Logger.Error("failed to set policy", zap.Uint32("code", res.TxResult.Code))
				return c.OpenRetentionPolicyActionSelector()
			} else {
				c.Logger.Error(
					"failed to set policy",
					zap.Uint32("code", res.TxResult.Code),
					zap.String("log", res.TxResult.Log),
				)
				return c.OpenRetentionPolicyActionSelector()
			}
		}
	}

	return c.OpenRetentionPolicyActionSelector()
}
