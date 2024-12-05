package handlers

import (
	"context"
	types "github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	"github.com/RyanW02/wineventchain/offchain-interface/internal/server"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/transport/payload"
	"go.uber.org/zap"
	"time"
)

func BroadcastHandler[T, U any](httpServer *server.Server[T, U]) payload.BroadcastHandler {
	return func(logger *zap.Logger, sourceName string, request types.SubmitRequest) {
		logger.Info(
			"Received broadcast event",
			zap.Stringer("event_id", request.EventId),
			zap.String("source", sourceName),
		)

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*15)
		defer cancelFunc()

		if err := httpServer.StoreEvent(ctx, request); err != nil {
			logger.Error("Failed to store event", zap.Error(err))
		}
	}
}
