package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/RyanW02/wineventchain/app/internal/config"
	"github.com/RyanW02/wineventchain/app/internal/utils"
	"github.com/RyanW02/wineventchain/app/pkg/events"
	"github.com/RyanW02/wineventchain/app/pkg/identity"
	"github.com/RyanW02/wineventchain/app/pkg/multiplexer"
	"github.com/RyanW02/wineventchain/app/pkg/retentionpolicy"
	dbm "github.com/cometbft/cometbft-db"
	"go.mongodb.org/mongo-driver/mongo"
	mongoOptions "go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/viper"

	abci "github.com/cometbft/cometbft/abci/types"
	cfg "github.com/cometbft/cometbft/config"
	tmflags "github.com/cometbft/cometbft/libs/cli/flags"
	"github.com/cometbft/cometbft/libs/log"
	nm "github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/proxy"
)

var (
	tendermintConfigPath = flag.String("tendermint_config", "$HOME/.cometbft/config/config.toml", "Path to tendermint config.toml")
	configPath           = flag.String("config", "", "Path to config.json")
)

func main() {
	flag.Parse()

	logConfig := zap.NewDevelopmentConfig()
	logConfig.DisableStacktrace = true
	logConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	logConfig.Encoding = "console"

	logger := utils.Must(logConfig.Build())

	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	var conf config.Config
	if configPath != nil && *configPath != "" {
		logger.Info("Loading config from file", zap.String("path", *configPath))

		conf, err = config.LoadJson(*configPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				logger.Error("config file does not exist", zap.String("path", *configPath))

				defaultMarshalled, err := json.MarshalIndent(config.Default(), "", "  ")
				if err != nil {
					logger.Fatal("failed to marshal default config", zap.Error(err))
				}

				if err := os.WriteFile(*configPath, defaultMarshalled, 0644); err != nil {
					logger.Fatal("failed to write default config file", zap.Error(err))
				}

				logger.Fatal("created default config file", zap.String("path", *configPath))
			} else {
				logger.Fatal("failed to load config", zap.Error(err))
			}
		}
	} else {
		logger.Info("Loading config from environment variables")

		conf, err = config.LoadEnv()
		if err != nil {
			logger.Fatal("failed to load config from environment variables", zap.Error(err))
		}
	}

	// Set up state store
	dbGenerator := newDbGenerator(logger, conf)
	stateDb := utils.Must(dbGenerator("state"))

	identityApp := utils.Must(identity.NewIdentityApp(logger, utils.Must(dbGenerator("identity"))))
	eventsApp := utils.Must(events.NewEventsApp(logger, utils.Must(dbGenerator("events")), identityApp.Repository))
	policyApp := utils.Must(retentionpolicy.NewRetentionPolicyApp(logger, identityApp.Repository, utils.Must(dbGenerator("retentionpolicy"))))
	app := multiplexer.NewApplication(
		logger,
		stateDb,
		identityApp,
		eventsApp,
		policyApp,
	)

	*tendermintConfigPath = strings.ReplaceAll(*tendermintConfigPath, "$HOME", homeDir)
	node, err := newNode(app, *tendermintConfigPath, cfg.DefaultDBProvider)
	if err != nil {
		logger.Fatal("failed to create new node", zap.Error(err))
	}

	if err := node.Start(); err != nil {
		logger.Fatal("failed to start CometBFT node", zap.Error(err))
	}

	defer func() {
		if err := node.Stop(); err != nil {
			logger.Error("failed to stop CometBFT node", zap.Error(err))
		}

		node.Wait()
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	os.Exit(0)
}

func newNode(app abci.Application, configFile string, dbProvider cfg.DBProvider) (*nm.Node, error) {
	config := cfg.DefaultConfig()
	config.RootDir = filepath.Dir(filepath.Dir(configFile))

	// Read config
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read cometbft config file: %w", err)
	}

	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cometbft config file: %w", err)
	}

	// Validate config
	if err := config.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("cometbft config is invalid: %w", err)
	}

	// Setup logger
	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))
	var err error
	logger, err = tmflags.ParseLogLevel(config.LogLevel, logger, cfg.DefaultLogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to parse log level: %w", err)
	}

	// Load private validator data
	pv := privval.LoadFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())

	// Read node key
	nodeKey, err := p2p.LoadNodeKey(config.NodeKeyFile())
	if err != nil {
		return nil, fmt.Errorf("failed to load node key: %w", err)
	}

	// Create node
	node, err := nm.NewNode(
		config,
		pv,
		nodeKey,
		proxy.NewLocalClientCreator(app),
		nm.DefaultGenesisDocProviderFunc(config),
		dbProvider,
		nm.DefaultMetricsProvider(config.Instrumentation),
		logger,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create new Tendermint node: %w", err)
	}

	return node, nil
}

func newDbGenerator(logger *zap.Logger, conf config.Config) func(name string) (dbm.DB, error) {
	switch conf.StateStore.Type {
	case config.StoreTypeDisk:
		if err := os.Mkdir(conf.StateStore.Disk.Directory, 0755); err != nil {
			if !errors.Is(err, os.ErrExist) {
				logger.Fatal("failed to create state store directory", zap.Error(err))
			}
		}

		return func(name string) (dbm.DB, error) {
			return dbm.NewGoLevelDB(name, conf.StateStore.Disk.Directory)
		}
	case config.StoreTypeMongoDB:
		serverAPI := mongoOptions.ServerAPI(mongoOptions.ServerAPIVersion1)
		opts := mongoOptions.Client().ApplyURI(conf.StateStore.Mongo.ConnectionString).SetServerAPIOptions(serverAPI)

		ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelFunc()

		mongoClient, err := mongo.Connect(ctx, opts)
		if err != nil {
			logger.Fatal("failed to connect to mongodb", zap.Error(err))
		}

		db := mongoClient.Database(conf.StateStore.Mongo.DatabaseName)
		return func(name string) (dbm.DB, error) {
			collection := db.Collection(name)
			return dbm.NewMongoDB(collection), nil
		}
	default:
		logger.Fatal("unknown state store type", zap.String("type", string(conf.StateStore.Type)))
		panic("unreachable")
	}
}
