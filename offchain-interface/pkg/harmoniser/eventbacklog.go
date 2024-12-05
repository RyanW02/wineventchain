package harmoniser

import (
	"context"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/state"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/transport/payload"
	"go.uber.org/zap"
	"time"
)

const OutboundFlushInterval = time.Second * 5

func (h *Harmoniser[T, U]) StartEventBackfillLoop() {
	shutdownCh := h.shutdownOrchestrator.Subscribe()
	timer := time.NewTicker(h.config.Backfill.EventPollInterval.Duration())

	for {
		select {
		case ch := <-shutdownCh:
			h.logger.Info("Shutting down event backfill loop")
			ch <- nil
			return
		case <-timer.C:
			h.logger.Debug("Checking for missing events")

			missingEventCount, err := h.state.MissingEventCount(context.Background())
			if err != nil {
				h.logger.Error("Failed to get missing event count from state DB", zap.Error(err))
			}

			if missingEventCount > 0 {
				h.logger.Info("Found missing events in state DB", zap.Int("count", missingEventCount))
			} else {
				h.logger.Debug("No missing events found")
			}

			var after *U
			for {
				missingEvents, err := h.state.MissingEvents(context.Background(), after, h.config.Backfill.EventFetchChunkSize)
				if err != nil {
					h.logger.Error("Failed to get missing events from state DB", zap.Error(err))
					continue
				}

				if len(missingEvents) == 0 {
					h.logger.Debug("No missing events found")
					break
				}

				h.logger.Info("Attempting to retrieve chunk of missing events", zap.Int("chunk_size", len(missingEvents)))

				// Request missing events in bulk over the network to reduce chatter
				var multicastBuffer, unicastBuffer []events.EventHash
				for _, ev := range missingEvents {
					// Processing events is a long task, so check for shutdown requests
					select {
					case ch := <-shutdownCh:
						h.logger.Info("Shutting down event backfill loop (mid-operation)")
						ch <- nil
						return
					default:
					}

					shouldRequest, useMulticast, err := h.handleMissingEvent(ev.MissingEvent)
					if err != nil {
						h.logger.Error("Failed to handle missing event", zap.Error(err), zap.Stringer("event_id", ev.EventId))
						continue
					}

					if shouldRequest {
						if useMulticast {
							multicastBuffer = append(multicastBuffer, ev.EventId)
						} else {
							unicastBuffer = append(unicastBuffer, ev.EventId)
						}
					}
				}

				after = &missingEvents[len(missingEvents)-1].Id

				if len(multicastBuffer) > 0 {
					h.logger.Info("Requesting missing events via multicast", zap.Int("count", len(multicastBuffer)), zap.Any("events", multicastBuffer))

					req := payload.NewEventRequest(multicastBuffer)
					marshalled, err := payload.NewPayloadMarshalled(payload.TypeRequestEvent, req)
					if err != nil {
						h.logger.Error(
							"Failed to marshal payload",
							zap.Error(err),
							zap.Stringers("event_ids", multicastBuffer),
							zap.Any("payload", req),
						)
						continue
					}

					for _, eventId := range multicastBuffer {
						if err := h.state.IncrementMissingEventRetryCount(context.Background(), eventId); err != nil {
							h.logger.Error(
								"Failed to increment missing event retry count",
								zap.Error(err),
								zap.Stringer("event_id", eventId),
							)
						}
					}

					if err := h.transport.Broadcast(marshalled); err != nil {
						h.logger.Error(
							"Failed to send multicast request",
							zap.Error(err),
							zap.Stringers("event_ids", multicastBuffer),
						)
						continue
					}

					select {
					case <-time.After(h.config.Backfill.MulticastBackoff.Duration()):
					case ch := <-shutdownCh:
						h.logger.Info("Shutting down event backfill loop (mid-operation)")
						ch <- nil
						return
					}
				}

				if len(unicastBuffer) > 0 {
					h.logger.Info("Requesting missing events via unicast", zap.Int("count", len(unicastBuffer)))

					req := payload.NewEventRequest(unicastBuffer)
					marshalled, err := payload.NewPayloadMarshalled(payload.TypeRequestEvent, req)
					if err != nil {
						h.logger.Error(
							"Failed to marshal payload",
							zap.Error(err),
							zap.Stringers("event_ids", unicastBuffer),
							zap.Any("payload", req),
						)
						continue
					}

					unicastCtx, unicastCancelFunc := context.WithTimeout(context.Background(), time.Second*10)
					defer unicastCancelFunc()

					for _, eventId := range unicastBuffer {
						if err := h.state.IncrementMissingEventRetryCount(context.Background(), eventId); err != nil {
							h.logger.Error(
								"Failed to increment missing event retry count",
								zap.Error(err),
								zap.Stringer("event_id", eventId),
							)
						}
					}

					if err := h.transport.UnicastRandomNeighbour(unicastCtx, marshalled); err != nil {
						h.logger.Error(
							"Failed to send unicast request",
							zap.Error(err),
							zap.Stringers("event_ids", unicastBuffer),
						)
						continue
					}

					select {
					case <-time.After(h.config.Backfill.UnicastBackoff.Duration()):
					case ch := <-shutdownCh:
						h.logger.Info("Shutting down event backfill loop (mid-operation)")
						ch <- nil
						return
					}
				}

				if len(missingEvents) < h.config.Backfill.EventFetchChunkSize {
					break
				}
			}
		}
	}
}

// handleMissingEvent returns a tuple (should_request, use_multicast, err), where should_request is a boolean
// indicating whether the event should be requested from the network and used_multicast is a boolean indicating whether
// the event should be requested over multicast, instead of a direct unicast request to another node.
func (h *Harmoniser[T, U]) handleMissingEvent(ev state.MissingEvent) (bool, bool, error) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	_, alreadyExists, err := h.repository.Events().GetEventById(ctx, ev.EventId)
	if err != nil {
		h.logger.Error("Failed to check if event exists in local repository", zap.Error(err))
		return false, false, err
	}
	cancelFunc()

	if alreadyExists {
		h.logger.Debug(
			"Event already exists in local repository, removing from missing events list",
			zap.Stringer("event_id", ev.EventId),
		)

		if err := h.state.RemoveMissingEvent(context.Background(), ev.EventId); err != nil {
			h.logger.Error("Failed to remove event from missing events list", zap.Error(err), zap.Stringer("event_id", ev.EventId))
			return false, false, err
		}

		return false, false, nil
	}

	if ev.ReceivedTime.After(time.Now().Add(-h.config.Backfill.NewEventIgnoreThreshold.Duration())) {
		h.logger.Debug(
			"Ignoring event as it's too new",
			zap.Stringer("event_id", ev.EventId),
			zap.Time("received_time", ev.ReceivedTime),
			zap.Time("now", time.Now()),
		)
		return false, false, nil
	}

	// If we haven't received the event after N retries, remove it from the missing events list - assume that
	// no off-chain interface nodes have the event data.
	if ev.RetryCount >= h.config.Backfill.EventMaxRetries {
		h.logger.Warn(
			"Event has reached max retry count, removing from missing events list",
			zap.Stringer("event_id", ev.EventId),
			zap.Time("received_at", ev.ReceivedTime),
			zap.Int("retry_count", ev.RetryCount),
		)

		if err := h.state.RemoveMissingEvent(context.Background(), ev.EventId); err != nil {
			h.logger.Error("Failed to remove event from missing events list", zap.Error(err), zap.Stringer("event_id", ev.EventId))
			return false, false, err
		}

		return false, false, nil
	}

	if time.Now().Before(ev.LastRetryTime.Add(h.config.Backfill.EventRetryInterval.Duration())) {
		h.logger.Debug(
			"Event has been retried recently, skipping",
			zap.Stringer("event_id", ev.EventId),
			zap.Time("last_retry_time", ev.LastRetryTime),
		)
		return false, false, nil
	}

	// Request event directly from another node directly first to avoid unnecessary chatter, but only try using
	// unicast once. If it fails, resort to using multicast.
	if h.config.Backfill.TryUnicastFirst && !ev.RetriedUnicast {
		h.logger.Debug("Requesting event from another node using unicast", zap.Stringer("event_id", ev.EventId))
		return true, false, nil
	} else { // If we've already tried unicast, resort to using multicast.
		h.logger.Debug("Resorting to using multicast to request event", zap.Stringer("event_id", ev.EventId))
		return true, true, nil
	}
}
