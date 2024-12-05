package config

import (
	"encoding/json"
	"github.com/caarlos0/env/v10"
	"os"
)

type (
	Config struct {
		Production bool     `json:"production" env:"PRODUCTION" envDefault:"false"`
		PrettyLogs bool     `json:"pretty_logs" env:"PRETTY_LOGS" envDefault:"false"`
		LogLevel   string   `json:"log_level" env:"LOG_LEVEL" envDefault:"info"`
		Server     Server   `json:"server" envPrefix:"SERVER_"`
		Frontend   Frontend `json:"frontend" envPrefix:"FRONTEND_"`
	}

	Server struct {
		Address string `json:"address" env:"ADDRESS"`
	}

	Frontend struct {
		BuildPath string `json:"build_path" env:"BUILD_PATH"`
		IndexFile string `json:"index_file" env:"INDEX_FILE"`
	}
)

func Load() (Config, error) {
	var conf Config

	// Try to load JSON config file, but fallback to environment variables if it does not exist
	if _, err := os.Stat("config.json"); err == nil {
		bytes, err := os.ReadFile("config.json")
		if err != nil {
			return Config{}, err
		}

		if err := json.Unmarshal(bytes, &conf); err != nil {
			return Config{}, err
		}

		return conf, nil
	}

	if err := env.Parse(&conf); err != nil {
		return Config{}, err
	}

	return conf, nil
}
