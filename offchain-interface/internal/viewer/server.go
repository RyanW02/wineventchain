package viewer

import (
	"github.com/RyanW02/wineventchain/common/pkg/broadcast"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/offchain-interface/internal/config"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/blockchain"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/repository"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Server struct {
	config               config.Config
	logger               *zap.Logger
	repository           repository.Repository
	blockchainClient     *blockchain.RoundRobinClient
	shutdownOrchestrator *broadcast.ErrorWaitChannel
	streamBroadcaster    *streamBroadcaster

	router *gin.Engine
}

func NewServer(
	cfg config.Config,
	logger *zap.Logger,
	repository repository.Repository,
	blockchainClient *blockchain.RoundRobinClient,
	shutdownOrchestrator *broadcast.ErrorWaitChannel,
) *Server {
	if cfg.Production {
		gin.SetMode(gin.ReleaseMode)
	}

	return &Server{
		config:               cfg,
		logger:               logger,
		repository:           repository,
		blockchainClient:     blockchainClient,
		shutdownOrchestrator: shutdownOrchestrator,
		streamBroadcaster:    newStreamBroadcaster(logger, shutdownOrchestrator),

		router: gin.Default(),
	}
}

func (s *Server) EventBroadcastChannel() chan events.StoredEvent {
	return s.streamBroadcaster.EventChannel()
}

func (s *Server) Run() {
	go s.streamBroadcaster.StartLoop()

	_ = s.router.SetTrustedProxies(nil)

	// Safe to use allow all requests with CORS, as we use authentication tokens stored in window.localStorage,
	// not cookies, meaning the tokens are unavailable in cross-origin contexts.
	s.router.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		AllowWebSockets:  true,
	}))

	authGroup := s.router.Group("/auth")
	authGroup.GET("/check-token", s.authenticate, s.checkTokenHandler)
	authGroup.POST("/challenge", s.challengeHandler)
	authGroup.POST("/challenge-response", s.challengeResponseHandler)

	eventsGroup := s.router.Group("/events")
	eventsGroup.POST("", s.authenticate, s.searchEventsHandler) // POST used to search events, as JSON body with filters is required
	eventsGroup.GET("/by-id/:id", s.authenticate, s.getEventHandler)
	eventsGroup.GET("/stream", s.eventWebsocketHandler)

	s.logger.Info("Starting viewer server", zap.String("address", s.config.ViewerServer.Address))

	if err := s.router.Run(s.config.ViewerServer.Address); err != nil {
		s.logger.Fatal("failed to start viewer server", zap.Error(err))
	}
}
