package main

import (
	"context"
	"errors"
	"github.com/RyanW02/wineventchain/chain-client/config"
	"github.com/RyanW02/wineventchain/chain-client/internal"
	"github.com/RyanW02/wineventchain/chain-client/prompt"
	"github.com/cometbft/cometbft/rpc/client/http"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

func main() {
	conf, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	logCfg := zap.NewDevelopmentConfig()
	logCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	logCfg.DisableStacktrace = false

	logger, err := logCfg.Build()
	if err != nil {
		panic(err)
	}

	tmClient, err := http.New(conf.CometBFT.Address, conf.CometBFT.WebsocketAddress)
	if err != nil {
		panic(err)
	}

	if _, err := tmClient.ABCIInfo(context.Background()); err != nil {
		panic(err)
	}

	client := internal.Client{
		Config: conf,
		Client: tmClient,
		Logger: logger,
	}

	// Set active principal
	if conf.Client.ActivePrivateKey != nil {
		privKeyFile, ok := conf.Client.PrivateKeyFiles[*conf.Client.ActivePrivateKey]
		if !ok {
			conf.Client.ActivePrivateKey = nil
			if err := conf.Write(); err != nil {
				panic(err)
			}
		}

		privKey, err := internal.LoadPrivateKey(privKeyFile)
		if err != nil {
			conf.Client.ActivePrivateKey = nil
			if err := conf.Write(); err != nil {
				panic(err)
			}
		}

		client.ActivePrincipal = conf.Client.ActivePrivateKey
		client.ActivePrivateKey = privKey
	}

	for {
		if err := client.OpenMainMenu(); err != nil {
			if errors.Is(err, promptui.ErrInterrupt) {
				os.Exit(0)
				return
			} else {
				if err := prompt.Display("Error", err.Error()); err != nil {
					panic(err)
				}
			}
		}
	}
}
