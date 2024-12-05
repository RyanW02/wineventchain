package transport

import "context"

type EventTransport interface {
	AddRxListener(listener RxListener)
	ClearListeners()
	Broadcast(bytes []byte) error
	Unicast(ctx context.Context, targetIdentifier string, bytes []byte) error
	UnicastRandomNeighbour(ctx context.Context, bytes []byte) error
	Identifier() string
	Shutdown() error
}

type RxListener func(source string, bytes []byte)
