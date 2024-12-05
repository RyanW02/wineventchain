package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	PrincipalName       string `json:"principal_name"`
	PrincipalPrivateKey string `json:"principal_private_key"`
}

const Path = "config.json"

func Load() (Config, error) {
	bytes, err := os.ReadFile(Path)
	if err != nil {
		return Config{}, err
	}

	var config Config
	if err := json.Unmarshal(bytes, &config); err != nil {
		return Config{}, err
	}

	return config, nil
}
