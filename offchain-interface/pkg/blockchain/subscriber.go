package blockchain

import (
	"encoding/hex"
	"fmt"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/state"
	"github.com/cometbft/cometbft/rpc/client/http"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"strconv"
	"time"
)

const SubscribeRetryInterval = 30 * time.Second
const ChannelCapacity = 128

func (c *RoundRobinClient) Subscribe(
	logger *zap.Logger,
	blockHeightCh chan int64,
	eventCh chan state.MissingEvent,
	shutdownCh chan chan error,
) {
	// Subscribe on all clients, to prevent missing events if 1 node acts maliciously
	clients := c.pool.GetAll(true)

	for _, client := range clients {
		go func(client http.HTTP) {
			err := errors.New("not nil")
			for err != nil {
				err = c.subscribe(logger, client, blockHeightCh, eventCh, shutdownCh)
				if err != nil {
					logger.Error(
						"Client failed to subscribe to blockchain events",
						zap.Error(err),
						zap.String("node_address", client.Remote()),
					)

					time.Sleep(SubscribeRetryInterval)
				}
			}
		}(client)
	}
}

func (c *RoundRobinClient) subscribe(
	logger *zap.Logger,
	client http.HTTP,
	blockHeightCh chan int64,
	eventCh chan state.MissingEvent,
	shutdownCh chan chan error,
) error {
	// Our implementation will automatically reconnect to the websockets if the connection is dropped.
	newBlockCh, chainEventCh := make(chan coretypes.ResultEvent, ChannelCapacity), make(chan coretypes.ResultEvent, ChannelCapacity)

	if err := c.ConnectWebsocket(client, newBlockCh, chainEventCh, shutdownCh); err != nil {
		return err
	}

	for {
		select {
		case ev, ok := <-newBlockCh:
			if !ok { // channel closed
				newBlockCh = nil
			}

			dataMap, ok := ev.Data.(map[string]any)
			if !ok {
				logger.Error("Received NewBlock event with unexpected data type", zap.Any("event", ev))
				continue
			}

			blockHeightStr, err := traverseJsonMap[string](dataMap, "value", "block", "header", "height")
			if err != nil {
				logger.Error("Failed to parse block height from NewBlock event", zap.Error(err), zap.Any("event", ev))
				continue
			}

			blockHeight, err := strconv.ParseInt(blockHeightStr, 10, 64)
			if err != nil {
				logger.Error("Failed to parse block height from NewBlock event", zap.Error(err), zap.Any("event", ev))
				continue
			}

			blockHeightCh <- blockHeight
		case ev, ok := <-chainEventCh:
			if !ok { // channel closed
				chainEventCh = nil
			}

			key := fmt.Sprintf("%s.%s", events.EventCreate, events.AttributeEventId)
			eventIdSlice, ok := ev.Events[key]
			if !ok || len(eventIdSlice) == 0 {
				logger.Error("Received event without event ID", mapToFields(ev.Events)...)
				continue
			}

			blockHeightSlice, ok := ev.Events["tx.height"]
			if !ok || len(blockHeightSlice) == 0 {
				logger.Error("Received event without block height", mapToFields(ev.Events)...)
				continue
			}

			blockHeight, err := strconv.ParseInt(blockHeightSlice[0], 10, 64)
			if err != nil {
				logger.Error("Failed to parse block height", zap.Error(err))
				continue
			}

			eventIdDecoded, err := hex.DecodeString(eventIdSlice[0])
			if err != nil {
				logger.Error("Failed to decode event ID from hex", zap.Error(err), zap.String("event_id_hex", eventIdSlice[0]))
				continue
			}

			eventId := events.EventHash(eventIdDecoded)
			missingEvent := state.NewMissingEvent(eventId, time.Now(), blockHeight)

			eventCh <- missingEvent
		}

		if newBlockCh == nil && chainEventCh == nil {
			break
		}
	}

	logger.Warn("Websocket connection closed")
	return nil
}

func mapToFields(m map[string][]string) []zap.Field {
	fields := make([]zap.Field, 0, len(m))
	for k, v := range m {
		fields = append(fields, zap.Strings(k, v))
	}
	return fields
}

func traverseJsonMap[T any](m map[string]any, fields ...string) (T, error) {
	working := m
	for i, field := range fields {
		if i == len(fields)-1 {
			val, ok := working[field].(T)
			if !ok {
				return *new(T), errors.Errorf("Field %s is not of type %T", field, val)
			}
			return val, nil
		}

		next, ok := working[field].(map[string]any)
		if !ok {
			return *new(T), errors.Errorf("Field %s is not a map", field)
		}
		working = next
	}

	return *new(T), errors.New("no value found")
}
