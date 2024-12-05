package viewer

import (
	"context"
	"encoding/hex"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"time"
)

func (s *Server) getEventHandler(c *gin.Context) {
	id, err := hex.DecodeString(c.Param("id"))
	if err != nil {
		// Use 404 instead of 400 on purpose
		c.JSON(http.StatusNotFound, gin.H{"error": "invalid id"})
		return
	}

	ctx, cancelFunc := context.WithTimeout(c, time.Second*10)
	defer cancelFunc()

	event, ok, err := s.repository.Events().GetEventById(ctx, id)
	if err != nil {
		s.logger.Error("failed to get event by id", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get event by id"})
		return
	}

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
		return
	}

	c.JSON(http.StatusOK, event)
}
