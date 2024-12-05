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

func (c *Client) HandleSeed() error {
	principal, err := prompt.Text("Principal", validate.LengthBetween(1, 255))
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
		Data(identity.RequestTypeSeed, identity.PayloadSeed{
			Principal: identity.Principal(principal),
			Key:       pub,
		}).
		Unsigned().
		Marshal()

	if err != nil {
		return err
	}

	res, err := helpers.BroadcastAndPollDefault(context.Background(), c.Client, marshalled)
	if err != nil {
		return err
	}

	// Check if the transaction was successful
	if res.TxResult.Code == 0 {
		c.Logger.Info("Successfully seeded identity")

		if err := WritePrivateKey(privKeyPath, priv); err != nil {
			return err
		}

		// Write privkey path to config
		c.Config.Client.PrivateKeyFiles[principal] = privKeyPath
		c.Config.Client.ActivePrivateKey = &principal

		c.ActivePrincipal = &principal
		c.ActivePrivateKey = priv

		if err := c.Config.Write(); err != nil {
			return err
		}
	} else {
		if len(res.TxResult.Log) == 0 {
			c.Logger.Warn("failed to seed identity", zap.Uint32("code", res.TxResult.Code))
			return c.OpenIdentityActionSelector()
		} else {
			c.Logger.Warn(
				"failed to seed identity",
				zap.Uint32("code", res.TxResult.Code),
				zap.String("log", res.TxResult.Log),
			)
			return c.OpenIdentityActionSelector()
		}
	}

	return c.OpenIdentityActionSelector()
}
