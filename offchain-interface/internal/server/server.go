package server

import (
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/offchain-interface/internal/config"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/blockchain"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/repository"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/state"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/transport"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Server[T any, U any] struct {
	config           config.Config
	logger           *zap.Logger
	blockchain       *blockchain.RoundRobinClient
	repository       repository.Repository
	transport        transport.EventTransport
	state            state.Store[T, U]
	eventBroadcastCh chan events.StoredEvent // For the viewer

	router *gin.Engine
}

func NewServer[T any, U any](
	cfg config.Config,
	logger *zap.Logger,
	blockchainClient *blockchain.RoundRobinClient,
	repository repository.Repository,
	transport transport.EventTransport,
	state state.Store[T, U],
	eventBroadcastCh chan events.StoredEvent,
) *Server[T, U] {
	if cfg.Production {
		gin.SetMode(gin.ReleaseMode)
	}

	return &Server[T, U]{
		config:           cfg,
		logger:           logger,
		blockchain:       blockchainClient,
		repository:       repository,
		transport:        transport,
		state:            state,
		eventBroadcastCh: eventBroadcastCh,

		router: gin.Default(),
	}
}

func (s *Server[T, U]) Run() error {
	_ = s.router.SetTrustedProxies(nil)

	s.router.GET("/event/:event_id", s.HandleGetEvent)
	s.router.GET("/status", s.HandleStatus)
	s.router.POST("/event", s.HandleSubmit)

	// Register development / debug endpoints
	if !s.config.Production {
		s.router.GET("/debug/stats", s.HandleStats)
	}

	return s.router.Run(s.config.Server.Address)
}
