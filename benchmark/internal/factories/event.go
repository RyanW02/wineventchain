package factories

import (
	"encoding/base64"
	"github.com/RyanW02/wineventchain/benchmark/internal/config"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
	"github.com/RyanW02/wineventchain/common/pkg/types/rpc"
	"github.com/google/uuid"
	"github.com/informalsystems/tm-load-test/pkg/loadtest"
)

type (
	EventClientFactory struct {
		config config.Config
	}

	EventClient struct {
		principalName string
		privateKey    []byte
	}
)

var _ loadtest.ClientFactory = (*EventClientFactory)(nil)
var _ loadtest.Client = (*EventClient)(nil)

func NewEventClientFactory(config config.Config) *EventClientFactory {
	return &EventClientFactory{
		config: config,
	}
}

func (f *EventClientFactory) ValidateConfig(cfg loadtest.Config) error {
	return nil
}

func (f *EventClientFactory) NewClient(cfg loadtest.Config) (loadtest.Client, error) {
	decoded, err := base64.StdEncoding.DecodeString(f.config.PrincipalPrivateKey)
	if err != nil {
		return nil, err
	}

	return &EventClient{
		principalName: f.config.PrincipalName,
		privateKey:    decoded,
	}, nil
}

func (c *EventClient) GenerateTx() ([]byte, error) {
	return rpc.NewBuilder().
		App(events.AppName).
		Data(events.RequestTypeCreate, events.CreateRequest{
			Event: RandomEvent(),
			Nonce: uuid.New(),
		}).
		Signed(identity.Principal(c.principalName), c.privateKey).
		Marshal()
}
