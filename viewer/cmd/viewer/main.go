package main

import (
	"github.com/RyanW02/wineventchain/viewer/internal/config"
	"github.com/RyanW02/wineventchain/viewer/internal/server"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"strings"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger := buildLogger(cfg)
	defer logger.Sync()

	server := server.NewServer(cfg, logger.With(zap.String("module", "server")))
	server.Start()
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
