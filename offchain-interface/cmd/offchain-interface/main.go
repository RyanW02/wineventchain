package main

import (
	"context"
	"github.com/RyanW02/wineventchain/common/pkg/broadcast"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/offchain-interface/internal/config"
	"github.com/RyanW02/wineventchain/offchain-interface/internal/server"
	"github.com/RyanW02/wineventchain/offchain-interface/internal/viewer"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/blockchain"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/harmoniser"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/repository"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/repository/mongodb"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/retention"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/state"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/transport"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/transport/payload"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/transport/payload/handlers"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/transport/swim"
	"github.com/cometbft/cometbft/rpc/client/http"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger := buildLogger(cfg)
	defer logger.Sync()

	shutdownOrchestrator := broadcast.NewErrorWaitChannel()

	blockchainClient := buildBlockchainClient(cfg, logger.With(zap.String("module", "blockchain")))
	defer blockchainClient.Close()

	stateStore, err := state.NewLevelDBStore(cfg)
	if err != nil {
		logger.Fatal("Failed to create state store", zap.Error(err))
	}
	defer stateStore.Close(context.Background())

	db := connectMongo(cfg, logger)
	repository := buildRepository(logger.With(zap.String("module", "repository")), db)

	retentionAgent := retention.NewAgent(
		cfg,
		logger.With(zap.String("module", "retention_agent")),
		blockchainClient,
		repository,
	)
	go retentionAgent.StartLoop(shutdownOrchestrator.Subscribe())

	// Join transport cluster for off-chain interface node communication
	var transportClient transport.EventTransport
	if len(cfg.Transport.Peers) > 0 {
		logger.Info("Joining SWIM transport cluster")
		swimTransport, err := swim.NewSWIMTransport(cfg, logger.With(zap.String("module", "transport")))
		if err != nil {
			logger.Fatal("Failed to setup SWIM transport", zap.Error(err))
		}

		go swimTransport.StartListener()
		transportClient = swimTransport
	} else {
		logger.Info("No peers specified, using no-op transport")
		transportClient = transport.NewNoopTransport()
	}

	harmoniser := harmoniser.NewHarmoniser[[16]byte, [16]byte](
		cfg,
		logger.With(zap.String("module", "harmoniser")),
		stateStore,
		repository,
		blockchainClient,
		transportClient,
	)
	harmoniser.Run()

	var eventBroadcastCh chan events.StoredEvent
	if cfg.ViewerServer.Enabled {
		logger := logger.With(zap.String("module", "viewer_server"))
		viewerServer := viewer.NewServer(cfg, logger, repository, blockchainClient, shutdownOrchestrator)
		eventBroadcastCh = viewerServer.EventBroadcastChannel()

		go viewerServer.Run()

		go func() {
			ticker := time.NewTicker(time.Minute)

			for {
				select {
				case ch := <-shutdownOrchestrator.Subscribe():
					ch <- nil
					return
				case <-ticker.C:
					ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
					if err := repository.Challenges().DropExpiredChallenges(ctx, cfg.ViewerServer.ChallengeLifetime.Duration()); err != nil {
						logger.Error("Failed to drop expired authentication challenges", zap.Error(err))
					}
					cancel()
				}
			}
		}()
	}

	// Build HTTP server - it has methods to process incoming requests, including some gossip traffic
	httpServer := server.NewServer[[16]byte, [16]byte](
		cfg,
		logger.With(zap.String("module", "server")),
		blockchainClient,
		repository,
		transportClient,
		stateStore,
		eventBroadcastCh,
	)

	// Handle SWIM gossip traffic
	decoder := payload.NewDecoder(logger.With(zap.String("module", "gossip_handler"))).
		WithBroadcastHandler(handlers.BroadcastHandler(httpServer)).
		WithRequestHandler(handlers.EventRequestHandler(cfg, repository, transportClient)).
		WithEventBackfillResponseHandler(handlers.BackfillResponseHandler[[16]byte, [16]byte](
			blockchainClient,
			repository,
			stateStore,
			shutdownOrchestrator.Subscribe(),
		))

	transportClient.AddRxListener(func(sourceName string, bytes []byte) {
		if err := decoder.HandleMessage(sourceName, bytes); err != nil {
			logger.With(zap.String("module", "decoder")).
				Error("Error decoding transport packet", zap.Error(err), zap.ByteString("packet", bytes))
		}
	})

	go func() {
		if err := httpServer.Run(); err != nil {
			logger.Fatal("Failed to run HTTP server", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	<-stop

	// Shutdown gracefully
	logger.Info("Received shutdown signal!")
	transportClient.ClearListeners()

	if err := shutdownOrchestrator.Await(time.Second * 5); err != nil {
		logger.Error("Failed to shutdown backfill response handler", zap.Error(err))
	} else {
		logger.Info("Backfill response handler shutdown successfully")
	}

	if err := harmoniser.Shutdown(); err != nil {
		logger.Error("Failed to shutdown harmoniser", zap.Error(err))
	} else {
		logger.Info("Harmoniser shutdown successfully")
	}

	if err := transportClient.Shutdown(); err != nil {
		logger.Error("Failed to shutdown transport", zap.Error(err))
	} else {
		logger.Info("Transport shutdown successfully")
	}
}

func buildLogger(cfg config.Config) *zap.Logger {
	var logCfg zap.Config
	if cfg.Production {
		logCfg = zap.NewProductionConfig()

		if cfg.PrettyLogs {
			logCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
			logCfg.Encoding = "console"
		}
	} else {
		logCfg = zap.NewDevelopmentConfig()
		logCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	switch strings.ToLower(cfg.LogLevel) {
	case "error":
		logCfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	case "warn":
		logCfg.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "info":
		logCfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "debug":
		logCfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	default:
		logCfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	logger, err := logCfg.Build()
	if err != nil {
		panic(err)
	}

	return logger
}

func buildBlockchainClient(cfg config.Config, logger *zap.Logger) *blockchain.RoundRobinClient {
	var clients []http.HTTP
	for _, nodeAddress := range cfg.Blockchain.NodeAddresses {
		client, err := http.New(nodeAddress, "/websocket")
		if err != nil {
			logger.Error("Failed to create blockchain client", zap.Error(err), zap.String("nodeAddress", nodeAddress))
			continue
		}

		clients = append(clients, *client)
	}

	if len(clients) < cfg.Blockchain.MinimumNodes {
		logger.Fatal(
			"minimum online node count not met",
			zap.Int("minimumNodes", cfg.Blockchain.MinimumNodes),
			zap.Int("onlineNodes", len(clients)),
		)
	}

	return blockchain.NewRoundRobinClient(cfg, logger.With(zap.String("module", "blockchain")), clients)
}

func connectMongo(cfg config.Config, logger *zap.Logger) *mongo.Database {
	opts := options.Client().
		ApplyURI(cfg.MongoDB.URI).
		SetServerAPIOptions(options.ServerAPI(options.ServerAPIVersion1))

	client, err := mongo.Connect(context.Background(), opts)
	if err != nil {
		logger.Fatal("failed to connect to MongoDB", zap.Error(err))
	}

	// Ping server
	if err := client.Ping(context.Background(), nil); err != nil {
		logger.Fatal("failed to ping MongoDB server", zap.Error(err))
	}

	return client.Database(cfg.MongoDB.DatabaseName)
}

func buildRepository(logger *zap.Logger, db *mongo.Database) repository.Repository {
	repo := mongodb.NewMongoRepository(logger, db)
	if err := repo.InitSchema(context.Background()); err != nil {
		logger.Fatal("failed to initialize MongoDB schema", zap.Error(err))
	}

	return repo
}
