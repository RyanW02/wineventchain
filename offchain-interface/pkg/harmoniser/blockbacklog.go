package harmoniser

import (
	"context"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/state"
	"go.uber.org/zap"
	"time"
)

func (h *Harmoniser[T, U]) StartBlockBackfillLoop() {
	shutdownCh := h.shutdownOrchestrator.Subscribe()

	for {
		timer := time.NewTicker(h.config.Backfill.BlockPollInterval.Duration())
		select {
		case ch := <-shutdownCh:
			h.logger.Info("Shutting down block backfill loop")
			ch <- nil
			return
		case <-timer.C:
			missingBlocks, err := h.state.MissingBlocks(context.Background())
			if err != nil {
				h.logger.Error("Failed to get missing blocks from state DB", zap.Error(err))
				continue
			}

			if len(missingBlocks) == 0 {
				continue
			}

			count := 0
			for _, blockRange := range missingBlocks {
				count += int(blockRange.High - blockRange.Low)
			}
			h.logger.Info("Found missing blocks in state DB", zap.Int("count", count))

		outer:
			for _, blockRange := range missingBlocks {
				select {
				case ch := <-shutdownCh:
					h.logger.Info("Shutting down block backfill loop (mid-operation)")
					ch <- nil
					return
				default:
				}

				page := 1

				var retrieved int
				events, totalCount, err := h.blockchainClient.SearchEvents(blockRange.Low, blockRange.High, page, h.config.Backfill.BlockFetchChunkSize)
				if err != nil {
					h.logger.Error("Failed to fetch missing events", zap.Error(err))
					continue
				}

				h.handleBacklogChunk(blockRange, events)

				retrieved += h.config.Backfill.BlockFetchChunkSize
				page++

				for retrieved < totalCount {
					h.logger.Info(
						"Got chunk of event transactions, still more to retrieve",
						zap.Int("retrieved", retrieved),
						zap.Int("total", totalCount),
					)

					events, _, err := h.blockchainClient.SearchEvents(blockRange.Low, blockRange.High, page, h.config.Backfill.BlockFetchChunkSize)
					if err != nil {
						h.logger.Error("Failed to fetch missing events", zap.Error(err))
						continue outer
					}

					h.handleBacklogChunk(blockRange, events)

					retrieved += h.config.Backfill.BlockFetchChunkSize
					page++
				}

				if err := h.state.RemoveMissingBlockRange(context.Background(), blockRange.Id); err != nil {
					h.logger.Error("Failed to remove missing block range from state DB", zap.Error(err))
				}
			}
		}
	}
}

func (h *Harmoniser[T, U]) handleBacklogChunk(blockRange state.BlockRangeWithId[T], events []state.MissingEvent) {
	for _, event := range events {
		if _, err := h.state.AddMissingEvents(context.Background(), event); err != nil {
			h.logger.Error("Failed to store missing event in state DB", zap.Error(err))
		}

		h.logger.Debug(
			"Added missing event to state DB",
			zap.String("event_id", event.EventId.String()),
			zap.Int64("block_height", event.BlockHeight),
		)
	}

	if len(events) > 0 {
		maxHeight := events[len(events)-1].BlockHeight

		// Remove if the entire block range has been processed
		if maxHeight >= blockRange.High-1 {
			if err := h.state.RemoveMissingBlockRange(context.Background(), blockRange.Id); err != nil {
				h.logger.Error("Failed to remove missing block range from state DB", zap.Error(err))
			}
			return
		} else { // Otherwise just increase the lower bound
			// Don't use maxHeight+1, as there may still be more transactions at that height
			updatedBlockRange := state.NewBlockRangeWithId(state.NewBlockRange(maxHeight, blockRange.High), blockRange.Id)
			if err := h.state.UpdateMissingBlockRange(context.Background(), updatedBlockRange); err != nil {
				h.logger.Error("Failed to update missing block range in state DB", zap.Error(err))
			}
		}
	}
}
