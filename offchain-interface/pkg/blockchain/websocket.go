package blockchain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/cometbft/cometbft/rpc/client/http"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"go.uber.org/zap"
	"net/url"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
	"sync"
	"time"
)

const (
	queryIdEvents = iota
	queryIdBlocks

	WebsocketReconnectBackoff = time.Second * 10
	WebsocketSubscribeTimeout = time.Second * 10
)

// CometBFT's own websocket implementation has a [severe bug](https://github.com/tendermint/tendermint/issues/6729),
// whereby events are dropped if they are received too quickly. As such, a custom websocket client has been
// implemented using nhooyr.io/websocket (which is a wrapper around gorilla/websocket) to avoid this issue.

func (c *RoundRobinClient) ConnectWebsocket(
	client http.HTTP,
	newBlockCh, eventCh chan coretypes.ResultEvent,
	innerShutdownCh chan chan error,
) error {
	wsUrl, err := c.websocketUrl(client.Remote())
	if err != nil {
		return err
	}

	var connMu sync.Mutex
	var conn *websocket.Conn

	shutdownCh := make(chan struct{})
	isShutdown := false

	go func() {
		errCh := <-innerShutdownCh

		connMu.Lock()
		isShutdown = true
		if conn != nil {
			errCh <- conn.Close(websocket.StatusNormalClosure, "application got shutdown signal")
		}
		connMu.Unlock()

		shutdownCh <- struct{}{}
	}()

	go func() {
		for {
			// Connect to the websocket
			for conn == nil {
				select {
				case <-shutdownCh:
					return
				default:
					c.logger.Debug("Connecting to blockchain websocket", zap.String("node_address", client.Remote()))

					wsConn, err := connectWs(wsUrl.String())
					if err != nil {
						c.logger.Error("Failed to connect to blockchain websocket", zap.Error(err))

						// Delay between each connection attempt
						select {
						case <-shutdownCh:
							return
						case <-time.After(WebsocketReconnectBackoff):
							continue
						}
					}

					wsConn.SetReadLimit(16 * 1024 * 1024)

					c.logger.Info("Connected to blockchain websocket", zap.String("node_address", client.Remote()))

					connMu.Lock()
					conn = wsConn
					connMu.Unlock()
				}
			}

			// Create subscriptions
			eventQuery := fmt.Sprintf(`%s.%s='%s'`, events.EventCreate, events.AttributeType, events.AttributeValueCreate)
			if err := subscribe(conn, queryIdEvents, eventQuery); err != nil {
				c.logger.Error("Failed to subscribe to blockchain events", zap.Error(err), zap.String("query", eventQuery))

				// Delay, as we haven't waited for any events yet
				select {
				case <-shutdownCh:
					return
				case <-time.After(WebsocketReconnectBackoff):
					continue
				}
			}

			blockQuery := "tm.event='NewBlock'"
			if err := subscribe(conn, queryIdBlocks, blockQuery); err != nil {
				c.logger.Error("Failed to subscribe to blockchain events", zap.Error(err), zap.String("query", blockQuery))

				// Delay, as we haven't waited for any events yet
				select {
				case <-shutdownCh:
					return
				case <-time.After(WebsocketReconnectBackoff):
					continue
				}
			}

			// Listen for events
			for {
				msgType, bytes, err := conn.Read(context.Background())
				if err != nil {
					connMu.Lock()
					conn = nil
					connMu.Unlock()

					var closeErr *websocket.CloseError
					if errors.As(err, &closeErr) {
						c.logger.Warn("Blockchain websocket connection closed", zap.Error(err))
					} else {
						// Don't print nasty error if the websocket has been closed gracefully
						connMu.Lock()
						if isShutdown {
							connMu.Unlock()
							return
						}
						connMu.Unlock()

						c.logger.Error("Failed to read from blockchain websocket, will reconnect", zap.Error(err))
					}

					break
				}

				if msgType != websocket.MessageText {
					c.logger.Warn("Received unexpected message type from blockchain websocket", zap.Int("message_type", int(msgType)))
					continue
				}

				var data EventRpcResponse
				if err := json.Unmarshal(bytes, &data); err != nil {
					c.logger.Error("Failed to unmarshal websocket message", zap.Error(err))
					continue
				}

				c.logger.Debug("Received message from blockchain websocket", zap.Any("message", data))

				if data.Result.Query == "" || data.Result.Events == nil {
					c.logger.Debug("Received blank query or events from blockchain websocket", zap.Any("message", data))
					continue
				}

				// Use goroutine to push to channel to avoid backpressure from the websocket
				switch data.Id {
				case queryIdEvents:
					go func(res coretypes.ResultEvent) {
						eventCh <- res
					}(data.Result)
				case queryIdBlocks:
					go func(res coretypes.ResultEvent) {
						newBlockCh <- res
					}(data.Result)
				default:
					c.logger.Warn("Received unexpected subscription ID from blockchain websocket", zap.Int("subscription_id", data.Id))
				}
			}
		}
	}()

	return nil
}

func (c *RoundRobinClient) websocketUrl(nodeAddress string) (*url.URL, error) {
	parsed, err := url.Parse(nodeAddress)
	if err != nil {
		return nil, err
	}

	if parsed.Scheme == "https" {
		parsed.Scheme = "wss"
	} else {
		parsed.Scheme = "ws"
	}

	parsed.Path = "/websocket"
	return parsed, nil
}

func subscribe(conn *websocket.Conn, id int, query string) error {
	ctx, cancel := context.WithTimeout(context.Background(), WebsocketSubscribeTimeout)
	defer cancel()

	req := newSubscribeRequest(id, query)
	return wsjson.Write(ctx, conn, req)
}

func connectWs(url string) (*websocket.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, url, nil)
	return conn, err
}
