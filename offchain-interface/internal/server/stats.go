package server

import (
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/state"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (s *Server[T, U]) HandleStats(c *gin.Context) {
	eventCount, err := s.repository.Events().EventCount(c)
	if err != nil {
		s.logger.Error("Failed to get event count", zap.Error(err))
		c.JSON(500, gin.H{"error": "failed to get event count"})
		return
	}

	missingEventCount, err := s.state.MissingEventCount(c)
	if err != nil {
		s.logger.Error("Failed to get missing event count", zap.Error(err))
		c.JSON(500, gin.H{"error": "failed to get missing event count"})
		return
	}

	lastSeenBlockHeight, err := s.state.LastSeenBlockHeight(c)
	if err != nil {
		s.logger.Error("Failed to get last seen block height", zap.Error(err))
		c.JSON(500, gin.H{"error": "failed to get last seen block height"})
		return
	}

	missingBlocksWithKey, err := s.state.MissingBlocks(c)
	if err != nil {
		s.logger.Error("Failed to get missing blocks", zap.Error(err))
		c.JSON(500, gin.H{"error": "failed to get missing blocks"})
		return
	}

	missingBlocks := make([]state.BlockRange, 0, len(missingBlocksWithKey))
	for _, block := range missingBlocksWithKey {
		missingBlocks = append(missingBlocks, block.BlockRange)
	}

	c.IndentedJSON(200, gin.H{
		"event_count":            eventCount,
		"missing_event_count":    missingEventCount,
		"last_seen_block_height": lastSeenBlockHeight,
		"missing_blocks":         missingBlocks,
	})
}
