package internal

import (
	"context"
	"github.com/RyanW02/wineventchain/chain-client/prompt"
)

func (c *Client) HandleABCIInfo() error {
	info, err := c.Client.ABCIInfo(context.Background())
	if err != nil {
		return err
	}

	if err := prompt.DisplayMarshalled("ABCI Info", info.Response); err != nil {
		return err
	}

	return c.OpenAppSelector()
}
