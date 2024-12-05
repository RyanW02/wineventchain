package config

type (
	Config struct {
		CometBFT CometBFT `json:"comet_bft"`
		Client   Client   `json:"client"`
	}

	CometBFT struct {
		Address          string `json:"address"`
		WebsocketAddress string `json:"websocket_address"`
	}

	Client struct {
		ActivePrivateKey *string           `json:"active_private_key"`
		PrivateKeyFiles  map[string]string `json:"private_key_files"`
	}
)
