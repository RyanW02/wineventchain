package internal

import (
	"github.com/RyanW02/wineventchain/chain-client/prompt"
	"github.com/RyanW02/wineventchain/chain-client/validate"
)

func (c *Client) HandleImportPrivateKey() error {
	principal, err := prompt.Text("Principal Name", validate.LengthBetween(1, 255))
	if err != nil {
		return err
	}

	privKeyPath, err := prompt.Text("Path to private key", validate.MinLength(1))
	if err != nil {
		return err
	}

	// Verify key exists
	if _, err := LoadPrivateKey(privKeyPath); err != nil {
		if err := prompt.Display("Failed to load private key", err.Error()); err != nil {
			return err
		}

		return c.OpenPrivateKeySelector()
	}

	c.Config.Client.PrivateKeyFiles[principal] = privKeyPath
	if err := c.Config.Write(); err != nil {
		return err
	}

	return c.OpenPrivateKeySelector()
}
