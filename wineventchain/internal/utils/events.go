package utils

import abci "github.com/cometbft/cometbft/abci/types"

func Event(eventName string, attributes ...abci.EventAttribute) abci.Event {
	return abci.Event{
		Type:       eventName,
		Attributes: attributes,
	}
}
