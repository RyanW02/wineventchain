package config

import (
	"encoding/json"
	"github.com/RyanW02/wineventchain/common/pkg/types"
	"github.com/caarlos0/env/v10"
	"os"
	"strings"
)

type (
	Config struct {
		Production     bool           `json:"production" env:"PRODUCTION" envDefault:"false"`
		PrettyLogs     bool           `json:"pretty_logs" env:"PRETTY_LOGS" envDefault:"false"`
		LogLevel       string         `json:"log_level" env:"LOG_LEVEL" envDefault:"info"`
		Server         Server         `json:"server" envPrefix:"SERVER_"`
		ViewerServer   ViewerServer   `json:"viewer_server" envPrefix:"VIEWER_SERVER_"`
		Blockchain     Blockchain     `json:"blockchain" envPrefix:"BLOCKCHAIN_"`
		MongoDB        MongoDB        `json:"mongodb" envPrefix:"MONGODB_"`
		State          State          `json:"state" envPrefix:"STATE_"`
		Transport      Transport      `json:"transport" envPrefix:"TRANSPORT_"`
		Backfill       Backfill       `json:"backfill" envPrefix:"BACKFILL_"`
		EventRetention EventRetention `json:"event_retention" envPrefix:"EVENT_RETENTION_"`
	}

	Server struct {
		Address string `json:"address" env:"ADDRESS"`
	}

	ViewerServer struct {
		Enabled           bool                     `json:"enabled" env:"ENABLED" envDefault:"true"`
		Address           string                   `json:"address" env:"ADDRESS" envDefault:"0.0.0.0:4000"`
		JWTAlgorithm      string                   `json:"jwt_algorithm" env:"JWT_ALGORITHM" envDefault:"HS256"`
		JWTSecret         string                   `json:"jwt_secret" env:"JWT_SECRET"`
		ChallengeLifetime types.MarshalledDuration `json:"challenge_lifetime" env:"CHALLENGE_LIFETIME" envDefault:"5m"`
		SearchPageLimit   int                      `json:"search_page_limit" env:"SEARCH_PAGE_LIMIT" envDefault:"15"`
	}

	Blockchain struct {
		NodeAddresses []string `json:"node_addresses" env:"NODE_ADDRESSES" envSeparator:","`
		MinimumNodes  int      `json:"minimum_nodes" env:"MINIMUM_NODES" envDefault:"1"`
	}

	MongoDB struct {
		URI          string `json:"uri" env:"URI"`
		DatabaseName string `json:"database_name" env:"DATABASE_NAME"`
	}

	State struct {
		Path string `json:"path" env:"PATH" envDefault:"state.db"`
	}

	Transport struct {
		NodeName             string      `json:"node_name" env:"NODE_NAME"`
		BindAddress          string      `json:"bind_address" env:"BIND_ADDRESS" envDefault:"0.0.0.0"`
		BindPort             int         `json:"bind_port" env:"BIND_PORT" envDefault:"7946"`
		NetworkType          NetworkType `json:"network_type" env:"NETWORK_TYPE" envDefault:"lan"`
		RetransmitMultiplier int         `json:"retransmit_multiplier" env:"RETRANSMIT_MULTIPLIER" envDefault:"2"`
		UseGossip            bool        `json:"use_gossip" env:"USE_GOSSIP" envDefault:"true"`
		Peers                []string    `json:"peers" env:"PEERS" envSeparator:","`
		UseEncryption        bool        `json:"use_encryption" env:"USE_ENCRYPTION" envDefault:"false"`
		SharedKey            string      `json:"shared_key" env:"SHARED_KEY"`
	}

	NetworkType string

	Backfill struct {
		TryUnicastFirst         bool                     `json:"try_unicast_first" env:"TRY_UNICAST_FIRST" envDefault:"true"`
		BlockPollInterval       types.MarshalledDuration `json:"block_poll_interval" env:"BLOCK_POLL_INTERVAL" envDefault:"1m"`
		BlockFetchChunkSize     int                      `json:"block_fetch_chunk_size" env:"BLOCK_FETCH_CHUNK_SIZE" envDefault:"100"`
		EventPollInterval       types.MarshalledDuration `json:"event_poll_interval" env:"EVENT_POLL_INTERVAL" envDefault:"1m"`
		EventFetchChunkSize     int                      `json:"event_fetch_chunk_size" env:"EVENT_FETCH_CHUNK_SIZE" envDefault:"100"`
		NewEventIgnoreThreshold types.MarshalledDuration `json:"event_new_event_ignore_threshold" env:"EVENT_NEW_EVENT_IGNORE_THRESHOLD" envDefault:"5m"`
		EventRetryInterval      types.MarshalledDuration `json:"event_retry_interval" env:"EVENT_RETRY_INTERVAL" envDefault:"30m"`
		EventMaxRetries         int                      `json:"event_max_retries" env:"EVENT_MAX_RETRIES" envDefault:"48"`
		MulticastBackoff        types.MarshalledDuration `json:"multicast_backoff" env:"MULTICAST_BACKOFF" envDefault:"5s"`
		UnicastBackoff          types.MarshalledDuration `json:"unicast_backoff" env:"UNICAST_BACKOFF" envDefault:"1s"`
	}

	EventRetention struct {
		RunAtStartup bool                     `json:"run_at_startup" env:"RUN_AT_STARTUP" envDefault:"false"`
		ScanInterval types.MarshalledDuration `json:"scan_interval" env:"SCAN_INTERVAL" envDefault:"1h"`
		ScanTimeout  types.MarshalledDuration `json:"scan_timeout" env:"SCAN_TIMEOUT" envDefault:"30m"`
	}
)

const (
	NetworkTypeWAN   NetworkType = "wan"
	NetworkTypeLAN   NetworkType = "lan"
	NetworkTypeLocal NetworkType = "local"
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

func (n NetworkType) ConvertCase() NetworkType {
	return NetworkType(strings.ToLower(n.String()))
}

func (n NetworkType) String() string {
	return string(n)
}
