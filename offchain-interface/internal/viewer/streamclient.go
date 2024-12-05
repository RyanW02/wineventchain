package viewer

import (
	"encoding/json"
	"errors"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/repository"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"sync"
	"time"
)

type streamClient struct {
	ws          *websocket.Conn
	server      *Server
	broadcaster *streamBroadcaster
	tx          chan []byte

	mu              sync.RWMutex
	isAuthenticated bool
	filters         []repository.Filter
}

type websocketMessage struct {
	Type    websocketMessageType `json:"type"`
	Payload json.RawMessage      `json:"data"`
}

type websocketMessageType string

type websocketErrorPayload struct {
	Message string `json:"message"`
}

const (
	wsReadLimit         = 8 * 1024
	wsWriteTimeout      = time.Second * 10
	wsKeepaliveInterval = time.Second * 30
	wsKeepaliveTimeout  = time.Second * 40

	wsMessageTypeAuth       websocketMessageType = "auth"
	wsMessageTypeSetFilters websocketMessageType = "subscribe"
	wsMessageTypeError      websocketMessageType = "error"
	wsMessageTypeEvent      websocketMessageType = "event"
)

func (s *Server) newStreamClient(broadcaster *streamBroadcaster, ws *websocket.Conn) *streamClient {
	return &streamClient{
		ws:              ws,
		server:          s,
		broadcaster:     broadcaster,
		tx:              make(chan []byte),
		mu:              sync.RWMutex{},
		isAuthenticated: false,
	}
}

func (c *streamClient) Configure() {
	c.ws.SetReadLimit(wsReadLimit)
	_ = c.ws.SetReadDeadline(time.Now().Add(wsKeepaliveTimeout))
	c.ws.SetPongHandler(func(string) error {
		_ = c.ws.SetReadDeadline(time.Now().Add(wsKeepaliveTimeout))
		return nil
	})
}

func (c *streamClient) Authenticated() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isAuthenticated
}

func (c *streamClient) Filters() []repository.Filter {
	copied := make([]repository.Filter, len(c.filters))

	c.mu.RLock()
	defer c.mu.RUnlock()

	copy(copied, c.filters)

	return copied
}

func (c *streamClient) Write(bytes []byte) {
	c.tx <- bytes
}

func (c *streamClient) WriteJSON(messageType websocketMessageType, data any) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	payload := websocketMessage{
		Type:    messageType,
		Payload: dataBytes,
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	c.Write(bytes)
	return nil
}

func (c *streamClient) WriteErrorAndClose(message string) error {
	payload := websocketErrorPayload{
		Message: message,
	}

	if err := c.WriteJSON(wsMessageTypeError, payload); err != nil {
		return err
	}

	return c.ws.Close()
}

func (c *streamClient) StartWriteLoop() {
	ticker := time.NewTicker(wsKeepaliveInterval)
	defer ticker.Stop()

	shutdownCh := c.server.shutdownOrchestrator.Subscribe()

	for {
		select {
		// The defer Close() will close the reader as well.
		case errCh := <-shutdownCh:
			errCh <- nil
			return
		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				// Socket likely closed
				c.server.logger.Debug("failed to write ping to websocket", zap.Error(err))
				return
			}
		case bytes := <-c.tx:
			_ = c.ws.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
			if err := c.ws.WriteMessage(websocket.TextMessage, bytes); err != nil {
				c.server.logger.Debug("failed to write message to websocket", zap.Error(err))
				return
			}
		}
	}
}

func (c *streamClient) StartReadLoop() {
	defer func() {
		_ = c.ws.Close()
		c.broadcaster.Unregister(c)
	}()

	for {
		msgType, bytes, err := c.ws.ReadMessage()
		if err != nil {
			var closeError *websocket.CloseError
			if !errors.As(err, &closeError) { // errors.Is comparison does not work for *websocket.CloseError
				c.server.logger.Warn(
					"failed to read message from websocket",
					zap.Error(err),
					zap.Stringer("client", c.ws.RemoteAddr()),
				)
			}

			return
		}

		if msgType != websocket.TextMessage {
			c.server.logger.Debug("received non-text message from websocket", zap.Stringer("client", c.ws.RemoteAddr()))
			return
		}

		c.handleMessage(bytes)
	}
}

func (c *streamClient) handleMessage(bytes []byte) {
	var message websocketMessage
	if err := json.Unmarshal(bytes, &message); err != nil {
		c.server.logger.Debug(
			"failed to unmarshal websocket message",
			zap.Error(err),
			zap.Stringer("client", c.ws.RemoteAddr()),
		)
		return
	}

	switch message.Type {
	case wsMessageTypeAuth:
		c.handleAuth(message.Payload)
	case wsMessageTypeSetFilters:
		c.handleSetFilters(message.Payload)
	}
}

func (c *streamClient) handleAuth(payload json.RawMessage) {
	var authPayload struct {
		Token string `json:"token"`
	}

	if err := json.Unmarshal(payload, &authPayload); err != nil {
		c.server.logger.Debug(
			"failed to unmarshal auth payload",
			zap.Error(err),
			zap.Stringer("client", c.ws.RemoteAddr()),
		)
		return
	}

	_, valid := c.server.validateToken([]byte(authPayload.Token))
	if !valid {
		if err := c.WriteErrorAndClose("invalid token"); err != nil {
			c.server.logger.Debug(
				"failed to write error message to websocket",
				zap.Error(err),
				zap.Stringer("client", c.ws.RemoteAddr()),
			)
		}

		return
	}

	c.mu.Lock()
	c.isAuthenticated = true
	c.mu.Unlock()
}

func (c *streamClient) handleSetFilters(payload json.RawMessage) {
	var filters []repository.Filter
	if err := json.Unmarshal(payload, &filters); err != nil {
		c.server.logger.Debug(
			"failed to unmarshal filter payload",
			zap.Error(err),
			zap.Stringer("client", c.ws.RemoteAddr()),
		)
		return
	}

	c.mu.Lock()
	c.filters = filters
	c.mu.Unlock()
}
