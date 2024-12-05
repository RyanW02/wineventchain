package handlers

import (
	"bytes"
	"context"
	"encoding/hex"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/blockchain"
	repository2 "github.com/RyanW02/wineventchain/offchain-interface/pkg/repository"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/state"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/transport/payload"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"time"
)

type backfillResponseMsg struct {
	logger     *zap.Logger
	sourceName string
	event      payload.EventBackfillResponseData
}

func BackfillResponseHandler[T any, U any](
	blockchainClient *blockchain.RoundRobinClient,
	repo repository2.Repository,
	state state.Store[T, U],
	shutdownTx chan chan error,
) payload.EventBackfillResponseHandler {
	eventCh := startBackfillResponseHandlerLoop(blockchainClient, repo, state, shutdownTx)

	return func(logger *zap.Logger, sourceName string, res payload.EventBackfillResponse) {
		logger.Debug("Received event backfill response", zap.Stringers("event_ids", res.EventIds()))

		for _, event := range res.Events {
			eventCh <- backfillResponseMsg{
				logger:     logger,
				sourceName: sourceName,
				event:      event,
			}
		}
	}
}

func startBackfillResponseHandlerLoop[T any, U any](
	blockchainClient *blockchain.RoundRobinClient,
	repo repository2.Repository,
	state state.Store[T, U],
	shutdownTx chan chan error,
) chan backfillResponseMsg {
	ch := make(chan backfillResponseMsg)

	go func() {
		for {
			// Check if the application has requested to shut down before processing each event, as
			// processing events is a long task.
			select {
			case errCh := <-shutdownTx:
				errCh <- nil
				return
			case msg := <-ch:
				logger := msg.logger
				sourceName := msg.sourceName
				event := msg.event

				logger.Debug("Processing backfilled event", zap.Stringer("event_id", event.EventId))

				// Fetch TX from blockchain
				tx, err := blockchainClient.GetEventByTx(event.TxHash)
				if err != nil {
					if errors.Is(err, blockchain.ErrEventNotFound) {
						logger.Warn(
							"Event not found in blockchain",
							zap.Stringer("event_id", event.EventId),
							zap.Stringer("tx_hash", event.TxHash),
							zap.String("source", sourceName),
						)
						return
					}

					logger.Error("Failed to get event metadata by tx", zap.Error(err))
					return
				}

				// Check that the on-chain hash matches the hash of the event data submitted
				hash := event.EventData.Hash()
				if tx.OffChainHash != hex.EncodeToString(hash) {
					logger.Warn(
						"event data does not match the on-chain hash",
						zap.Stringer("tx_hash", event.TxHash),
						zap.Stringer("event_id", tx.Metadata.EventId),
						zap.String("on_chain_hash", tx.OffChainHash),
						zap.String("submitted_hash", hex.EncodeToString(hash)),
						zap.String("source", sourceName),
					)
					return
				}

				// Check we are talking about the same event
				if !bytes.Equal(tx.Metadata.EventId, event.EventId) {
					logger.Warn(
						"Event id does not match",
						zap.Stringer("tx_hash", event.TxHash),
						zap.Stringer("event_id", tx.Metadata.EventId),
						zap.Stringer("provided_event_id", event.EventId),
						zap.String("source", sourceName),
					)
					return
				}

				fullEvent := events.StoredEvent{
					EventWithData: events.EventWithData{
						Event:     tx.Event,
						EventData: event.EventData,
					},
					Metadata: tx.Metadata,
					TxHash:   event.TxHash,
				}

				ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
				if err := repo.Events().Store(ctx, fullEvent); err != nil {
					cancelFunc()

					if errors.Is(err, repository2.ErrEventAlreadyStored) {
						// Just log, don't throw error
						logger.Debug("Received duplicate event", zap.Stringer("event_id", event.EventId))
					} else {
						logger.Error("Failed to store event in repository", zap.Error(err))
						return
					}
				}
				cancelFunc()

				// Remove missing event marker from state store
				if err := state.RemoveMissingEvent(context.Background(), tx.Metadata.EventId); err != nil {
					// If there is an error, the harmoniser will realise the event is already stored on the next run,
					// and remove the missing event marker from the state store.
					logger.Error("Failed to remove missing event from state DB", zap.Error(err))
					return
				}

				logger.Info(
					"Event backfilled successfully",
					zap.Stringer("event_id", event.EventId),
					zap.Stringer("tx_hash", event.TxHash),
				)
			}
		}
	}()

	return ch
}
