package viewer

import (
	"context"
	"encoding/hex"
	"errors"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/common/pkg/utils"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"strconv"
	"time"
)

type searchRequestBody struct {
	Filters []repository.Filter `json:"filters"`
}

func (s *Server) searchEventsHandler(c *gin.Context) {
	var body searchRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	var firstEventId *events.EventHash
	if id, ok := c.GetQuery("first"); ok {
		idParsed, err := hex.DecodeString(id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 'first' query parameter"})
			return
		}

		firstEventId = utils.Ptr(events.EventHash(idParsed))
	}

	var page int
	if pageStr, ok := c.GetQuery("page"); ok {
		pageNum, err := strconv.Atoi(pageStr)
		if err == nil {
			page = pageNum - 1 // In the frontend, pages are 1-indexed
		}
	}

	ctx, cancelFunc := context.WithTimeout(c, time.Second*10)
	defer cancelFunc()

	results, err := s.repository.Events().SearchEvents(ctx, body.Filters, s.config.ViewerServer.SearchPageLimit, page, firstEventId)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidFilter) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}) // Safe to expose
			return
		}

		s.logger.Error("failed to search events", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to search events"})
		return
	}

	// Don't return `null` if no results are found
	if results == nil {
		results = make([]events.StoredEvent, 0)
	}

	c.JSON(http.StatusOK, results)
}
