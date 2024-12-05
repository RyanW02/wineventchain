package server

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (s *Server[T, U]) HandleStatus(ctx *gin.Context) {
	if err := s.repository.TestConnection(); err != nil {
		s.logger.Error("failed to connect to the database", zap.Error(err))
		ctx.JSON(500, gin.H{"status": "error", "error": "failed to connect to the database"})
		return
	}

	ctx.JSON(200, gin.H{"status": "ok"})
}
