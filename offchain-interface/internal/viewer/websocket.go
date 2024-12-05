package viewer

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"net/http"
	"time"
)

var upgrader = websocket.Upgrader{
	HandshakeTimeout: time.Second * 5,
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
	Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)

		marshalled, err := json.Marshal(gin.H{"error": reason.Error()})
		if err != nil {
			return
		}

		_, _ = w.Write(marshalled)
	},
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins
		return true
	},
	EnableCompression: true,
}

func (s *Server) eventWebsocketHandler(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		s.logger.Error("failed to upgrade websocket connection", zap.Error(err))
		return
	}

	client := s.newStreamClient(s.streamBroadcaster, ws)
	client.Configure()

	s.streamBroadcaster.Register(client)

	go client.StartReadLoop()
	go client.StartWriteLoop()
}
