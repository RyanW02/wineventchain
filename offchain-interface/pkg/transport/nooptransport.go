package transport

import "context"

// NoopTransport is a no-op implementation of the EventTransport interface. It does not do anything
// with the events it receives. It is useful for testing, and in cases where a node is started without
// any peers.
type NoopTransport struct{}

var _ EventTransport = (*NoopTransport)(nil)

func NewNoopTransport() *NoopTransport {
	return &NoopTransport{}
}

func (t *NoopTransport) AddRxListener(_ RxListener) {}

func (t *NoopTransport) ClearListeners() {}

func (t *NoopTransport) Broadcast(_ []byte) error {
	return nil
}

func (t *NoopTransport) Unicast(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (t *NoopTransport) UnicastRandomNeighbour(_ context.Context, _ []byte) error {
	return nil
}

func (t *NoopTransport) Identifier() string {
	return ""
}

func (t *NoopTransport) Shutdown() error {
	return nil
}
