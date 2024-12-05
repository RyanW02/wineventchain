package server

import (
	"github.com/RyanW02/wineventchain/viewer/internal/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"path"
)

type Server struct {
	config config.Config
	logger *zap.Logger
	router *gin.Engine
}

func NewServer(config config.Config, logger *zap.Logger) *Server {
	return &Server{
		config: config,
		logger: logger,
		router: gin.Default(),
	}
}

func (s *Server) Start() {
	s.router.Static("/_app", path.Join(s.config.Frontend.BuildPath, "/_app"))
	s.router.StaticFile("/style.css", path.Join(s.config.Frontend.BuildPath, "/style.css"))

	s.router.NoRoute(func(c *gin.Context) {
		c.File(path.Join(s.config.Frontend.BuildPath, s.config.Frontend.IndexFile))
	})

	s.logger.Info("Starting HTTP server", zap.String("address", s.config.Server.Address))

	if err := s.router.Run(s.config.Server.Address); err != nil {
		s.logger.Fatal("Failed to start server", zap.Error(err))
	}
}
