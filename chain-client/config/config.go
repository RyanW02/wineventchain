package config

import (
	"encoding/json"
	"os"
)

func LoadConfig() (Config, error) {
	conf := Default()

	// Write default config if it doesn't exist
	if _, err := os.Stat("config.json"); os.IsNotExist(err) {
		if err := conf.Write(); err != nil {
			return Config{}, err
		}
	}

	// DecodeFrames config from disk
	file, err := os.Open("config.json")
	if err != nil {
		return Config{}, err
	}

	if err := json.NewDecoder(file).Decode(&conf); err != nil {
		return Config{}, err
	}

	return conf, nil
}

func (c *Config) Write() error {
	file, err := os.Create("config.json")
	if err != nil {
		return err
	}

	bytes, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	if _, err := file.Write(bytes); err != nil {
		return err
	}

	return nil
}

func Default() Config {
	return Config{
		CometBFT: CometBFT{
			Address:          "http://localhost:26657/",
			WebsocketAddress: "ws://localhost:26657/websocket",
		},
		Client: Client{
			PrivateKeyFiles: make(map[string]string),
		},
	}
}
