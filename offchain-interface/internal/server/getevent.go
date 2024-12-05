package server

import (
	"context"
	"encoding/hex"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	types "github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"regexp"
	"time"
)

var sha256Regex = regexp.MustCompile("^[a-f0-9]{64}$")

func (s *Server[T, U]) HandleGetEvent(c *gin.Context) {
	eventIdStr := c.Param("event_id")
	if !sha256Regex.MatchString(eventIdStr) {
		c.JSON(400, gin.H{"error": "invalid event id"})
		return
	}

	eventIdRaw, err := hex.DecodeString(eventIdStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid event id"})
		return
	}

	eventId := events.EventHash(eventIdRaw)

	ctx, cancelFunc := context.WithTimeout(c, time.Second*5)
	defer cancelFunc()

	ev, found, err := s.repository.Events().GetEventById(ctx, eventId)
	if err != nil {
		s.logger.Error("failed to get event", zap.Stringer("event_id", eventId), zap.Error(err))
		c.JSON(500, gin.H{"error": "failed to get event"})
		return
	}

	if !found {
		c.JSON(404, gin.H{})
		return
	}

	c.JSON(200, types.GetEventResponse{
		Event: ev,
	})
}
