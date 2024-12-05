package internal

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"github.com/RyanW02/wineventchain/chain-client/prompt"
	"github.com/RyanW02/wineventchain/chain-client/validate"
	"github.com/RyanW02/wineventchain/common/pkg/blockchain/helpers"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
	"github.com/RyanW02/wineventchain/common/pkg/types/rpc"
	"go.uber.org/zap"
)

func (c *Client) HandleRegister() error {
	if c.ActivePrincipal == nil || c.ActivePrivateKey == nil {
		if err := prompt.Display("Error", "No active principal"); err != nil {
			return err
		}

		return c.OpenIdentityActionSelector()
	}

	principal, err := prompt.Text("Principal", validate.LengthBetween(1, 255))
	if err != nil {
		return err
	}

	role, err := prompt.Select("Choose a role", identity.RoleUser, identity.RoleAdmin)
	if err != nil {
		return err
	}

	privKeyPath, err := prompt.TextWithDefault("Path to write private key to", "./privkey_"+principal, validate.MinLength(1))
	if err != nil {
		return err
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	marshalled, err := rpc.NewBuilder().
		App(identity.AppName).
		Data(identity.RequestTypeRegister, identity.PayloadRegister{
			Principal: identity.Principal(principal),
			Role:      *role,
			Key:       pub,
		}).
		Signed(identity.Principal(*c.ActivePrincipal), c.ActivePrivateKey).
		Marshal()

	if err != nil {
		return err
	}

	res, err := helpers.BroadcastAndPollDefault(context.Background(), c.Client, marshalled)
	if err != nil {
		return err
	}

	// Check if the transaction was successful
	if res.TxResult.Code == identity.CodeOk {
		c.Logger.Info("Successfully registered identity")

		if err := WritePrivateKey(privKeyPath, priv); err != nil {
			return err
		}

		// Write privkey path to config
		c.Config.Client.PrivateKeyFiles[principal] = privKeyPath
		if err := c.Config.Write(); err != nil {
			return err
		}
	} else {
		if len(res.TxResult.Log) == 0 {
			c.Logger.Error("failed to register identity", zap.Uint32("code", res.TxResult.Code))
			return c.OpenIdentityActionSelector()
		} else {
			c.Logger.Error(
				"failed to register identity",
				zap.Uint32("code", res.TxResult.Code),
				zap.String("log", res.TxResult.Log),
			)
			return c.OpenIdentityActionSelector()
		}
	}

	return c.OpenIdentityActionSelector()
}
