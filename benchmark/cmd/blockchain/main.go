package main

import (
	"github.com/RyanW02/wineventchain/benchmark/internal/config"
	"github.com/RyanW02/wineventchain/benchmark/internal/factories"
	"github.com/informalsystems/tm-load-test/pkg/loadtest"
)

func main() {
	config, err := config.Load()
	if err != nil {
		panic(err)
	}

	factory := factories.NewEventClientFactory(config)

	if err := loadtest.RegisterClientFactory("events", factory); err != nil {
		panic(err)
	}

	loadtest.Run(&loadtest.CLIConfig{
		AppName:              "blockchain-benchmark",
		AppShortDesc:         "Benchmarks the events blockchain ABCI app",
		AppLongDesc:          "Generates and submits transactions to the events blockchain ABCI app",
		DefaultClientFactory: "events",
	})
}
