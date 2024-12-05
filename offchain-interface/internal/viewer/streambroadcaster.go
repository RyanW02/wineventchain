package viewer

import (
	"encoding/json"
	"github.com/RyanW02/wineventchain/common/pkg/broadcast"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/repository"
	"go.uber.org/zap"
	"sync"
)

type streamBroadcaster struct {
	logger     *zap.Logger
	clients    map[*streamClient]struct{} // Use a hashset, as order does not matter
	mu         sync.RWMutex
	ch         chan events.StoredEvent
	shutdownCh chan chan error
}

func newStreamBroadcaster(logger *zap.Logger, shutdownOrchestrator *broadcast.ErrorWaitChannel) *streamBroadcaster {
	return &streamBroadcaster{
		logger:     logger,
		clients:    make(map[*streamClient]struct{}),
		mu:         sync.RWMutex{},
		ch:         make(chan events.StoredEvent),
		shutdownCh: shutdownOrchestrator.Subscribe(),
	}
}

func (b *streamBroadcaster) Register(client *streamClient) {
	b.mu.Lock()
	b.clients[client] = struct{}{}
	b.mu.Unlock()
}

func (b *streamBroadcaster) Unregister(client *streamClient) {
	b.mu.Lock()
	delete(b.clients, client)
	b.mu.Unlock()
}

func (b *streamBroadcaster) EventChannel() chan events.StoredEvent {
	return b.ch
}

func (b *streamBroadcaster) StartLoop() {
	for {
		select {
		case errCh := <-b.shutdownCh:
			//b.mu.Lock()
			//for client := range b.clients {
			//	go client.ws.Close()
			//}
			//b.mu.Unlock()

			errCh <- nil
			return
		case event := <-b.ch:
			eventMarshalled, err := json.Marshal(event)
			if err != nil {
				b.logger.Error("failed to marshal event", zap.Error(err))
				continue
			}

			payload := websocketMessage{
				Type:    wsMessageTypeEvent,
				Payload: eventMarshalled,
			}

			payloadMarshalled, err := json.Marshal(payload)
			if err != nil {
				b.logger.Error("failed to marshal event payload", zap.Error(err))
				continue
			}

			b.mu.RLock()
			for client := range b.clients {
				if !client.Authenticated() {
					continue
				}

				matches, err := filtersMatch(event, client.Filters())
				if err != nil {
					// Debug severity, as this is usually as a result of invalid filters being set by the client
					b.logger.Debug("failed to match event with client filters", zap.Error(err))
					continue
				}

				if matches {
					client.Write(payloadMarshalled)
				}
			}
			b.mu.RUnlock()
		}
	}
}

func filtersMatch(event events.StoredEvent, filters []repository.Filter) (bool, error) {
	for _, filter := range filters {
		matches, err := filter.Matches(event)
		if err != nil {
			return false, err
		}

		if !matches {
			return false, nil
		}
	}

	return true, nil
}
