package internal

import (
	"crypto/ed25519"
	"github.com/RyanW02/wineventchain/chain-client/config"
	"github.com/cometbft/cometbft/rpc/client/http"
	"go.uber.org/zap"
)

type Client struct {
	Config config.Config
	Client *http.HTTP
	Logger *zap.Logger

	ActivePrincipal  *string
	ActivePrivateKey ed25519.PrivateKey
}
