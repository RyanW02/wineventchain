package harmoniser

import (
	"context"
	"github.com/RyanW02/wineventchain/common/pkg/broadcast"
	"github.com/RyanW02/wineventchain/offchain-interface/internal/config"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/blockchain"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/repository"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/state"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/transport"
	"go.uber.org/zap"
	"time"
)

// Harmoniser is responsible for ensuring that the off-chain node has all event data available.
// If the off-chain node is offline for any period of time, it will request missing events from other nodes.
type Harmoniser[T any, U any] struct {
	config               config.Config
	logger               *zap.Logger
	state                state.Store[T, U]
	repository           repository.Repository
	blockchainClient     *blockchain.RoundRobinClient
	transport            transport.EventTransport
	shutdownOrchestrator *broadcast.ErrorWaitChannel
}

func NewHarmoniser[T any, U any](
	cfg config.Config,
	logger *zap.Logger,
	state state.Store[T, U],
	repository repository.Repository,
	blockchainClient *blockchain.RoundRobinClient,
	transport transport.EventTransport,
) *Harmoniser[T, U] {
	return &Harmoniser[T, U]{
		config:               cfg,
		logger:               logger,
		state:                state,
		repository:           repository,
		blockchainClient:     blockchainClient,
		transport:            transport,
		shutdownOrchestrator: broadcast.NewErrorWaitChannel(),
	}
}

func (h *Harmoniser[T, U]) Run() {
	go h.ListenForMissedItems()
	go h.StartBlockBackfillLoop()
	go h.StartEventBackfillLoop()
}

func (h *Harmoniser[T, U]) Shutdown() error {
	h.logger.Info("Shutting down harmoniser")
	return h.shutdownOrchestrator.Await(time.Second * 15)
}

func (h *Harmoniser[T, U]) ListenForMissedItems() {
	shutDownCh := h.shutdownOrchestrator.Subscribe()
	blockHeightCh := make(chan int64)
	eventIdCh := make(chan state.MissingEvent)

	h.blockchainClient.
		Subscribe(h.logger.With(zap.String("module", "websocket")), blockHeightCh, eventIdCh, h.shutdownOrchestrator.Subscribe())

	for {
		select {
		case ch := <-shutDownCh:
			h.logger.Info("Shutting down blockchain event listener")
			ch <- nil
			return
		case blockHeight := <-blockHeightCh:
			h.logger.Debug("Received new block height", zap.Int64("block_height", blockHeight))

			lastBlockHeight, err := h.state.LastSeenBlockHeight(context.Background())
			if err != nil {
				h.logger.Error("Failed to get last seen block height", zap.Error(err))
				continue
			}

			if lastBlockHeight == nil && blockHeight > 0 {
				h.logger.Warn("No last seen block height found, we have missing blocks", zap.Int64("block_height", blockHeight))

				// Calculate missing blocks
				blockRange := state.NewBlockRange(0, blockHeight) // High is exclusive
				if _, err := h.state.AddMissingBlocks(context.Background(), blockRange); err != nil {
					h.logger.Error("Failed to store missing blocks in state DB", zap.Error(err))
				}
			} else {
				// If we've seen a higher block, skip and don't write to the state store
				if *lastBlockHeight >= blockHeight {
					continue
				}

				// If this is not last block + 1, we have missing blocks
				if *lastBlockHeight+1 != blockHeight {
					h.logger.Info("We have missing blocks", zap.Int64("last_seen_block_height", *lastBlockHeight), zap.Int64("current_block_height", blockHeight))

					blockRange := state.NewBlockRange(*lastBlockHeight+1, blockHeight) // High is exclusive
					if _, err := h.state.AddMissingBlocks(context.Background(), blockRange); err != nil {
						h.logger.Error("Failed to store missing blocks in state DB", zap.Error(err))
					}
				}
			}

			// Update last seen block height
			if err := h.state.SetLastSeenBlockHeight(context.Background(), blockHeight); err != nil {
				h.logger.Error("Failed to set last seen block height", zap.Error(err))
				continue
			}
		case ev := <-eventIdCh:
			h.logger.Info("Seen new event on blockchain", zap.Stringer("event_id", ev.EventId))

			if _, err := h.state.AddMissingEvents(context.Background(), ev); err != nil {
				h.logger.Error("Failed to add missing event", zap.Error(err))
			}
		}
	}
}
