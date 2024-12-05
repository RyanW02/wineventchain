package handlers

import (
	"context"
	"github.com/RyanW02/wineventchain/offchain-interface/internal/config"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/repository"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/transport"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/transport/payload"
	"go.uber.org/zap"
	"time"
)

func EventRequestHandler(cfg config.Config, repository repository.Repository, transport transport.EventTransport) payload.EventRequestHandler {
	return func(logger *zap.Logger, sourceName string, request payload.EventRequest) {
		logger.Info(
			"Received event backfill request",
			zap.Int("count", len(request.EventIds)),
			zap.String("source", sourceName),
		)

		logger.Debug("Event backfill request", zap.Stringers("event_ids", request.EventIds))

		// If we have made the event request, it is clear that we do not have the event.
		if cfg.Transport.NodeName == sourceName {
			logger.Debug("Ignoring event request from self", zap.String("source", sourceName))
			return
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
		defer cancelFunc()

		events, err := repository.Events().GetEventsById(ctx, request.EventIds)
		if err != nil {
			logger.Error("Failed to get event from repository", zap.Error(err))
			return
		}
		cancelFunc()

		logger.Info(
			"Retrieved events from repository",
			zap.Int("retrieved_count", len(events)),
			zap.Int("requested_count", len(request.EventIds)),
			zap.String("requester", sourceName),
		)

		if len(events) == 0 {
			logger.Debug(
				"No events found for request",
				zap.Stringers("requested_event_ids", request.EventIds),
				zap.String("source", sourceName),
			)
			return
		}

		data := make([]payload.EventBackfillResponseData, len(events))
		for i, event := range events {
			data[i] = payload.EventBackfillResponseData{
				EventId:   event.Metadata.EventId,
				TxHash:    event.TxHash,
				EventData: event.EventWithData.EventData,
			}
		}

		marshalled, err := payload.NewPayloadMarshalled(payload.TypeBackfillResponse, payload.EventBackfillResponse{
			Events: data,
		})

		unicastCtx, unicastCancelFunc := context.WithTimeout(context.Background(), time.Second*10)
		defer unicastCancelFunc()

		if err := transport.Unicast(unicastCtx, sourceName, marshalled); err != nil {
			logger.Error("Failed to unicast event backfill response", zap.Error(err), zap.Stringers("event_ids", request.EventIds))
			return
		}

		logger.Info(
			"Successfully responded to event backfill request",
			zap.Int("count", len(events)),
			zap.String("source", sourceName),
		)

		logger.Debug(
			"Successfully responded to event backfill request",
			zap.Stringers("requested_event_ids", request.EventIds),
			zap.Int("retrieved_count", len(events)),
			zap.String("source", sourceName),
		)
	}
}
