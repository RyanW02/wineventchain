package retention

import (
	"context"
	"github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	"github.com/RyanW02/wineventchain/offchain-interface/internal/config"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/blockchain"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/repository"
	"go.uber.org/zap"
	"time"
)

type Agent struct {
	config           config.Config
	logger           *zap.Logger
	blockchainClient *blockchain.RoundRobinClient
	repository       repository.Repository

	policy *offchain.StoredPolicy
}

func NewAgent(
	config config.Config,
	logger *zap.Logger,
	blockchainClient *blockchain.RoundRobinClient,
	repository repository.Repository,
) *Agent {
	return &Agent{
		config:           config,
		logger:           logger,
		blockchainClient: blockchainClient,
		repository:       repository,
	}
}

func (a *Agent) StartLoop(shutdownCh chan chan error) {
	ticker := time.NewTicker(a.config.EventRetention.ScanInterval.Duration())

	if a.config.EventRetention.RunAtStartup {
		if err := a.scanAndDrop(); err != nil {
			a.logger.Error("Failed to run retention policy scan at startup", zap.Error(err))
		}
	}

	for {
		select {
		case ch := <-shutdownCh:
			ch <- nil
			return
		case <-ticker.C:
			if err := a.scanAndDrop(); err != nil {
				a.logger.Error("Failed to run retention policy scan", zap.Error(err))
			}
		}
	}
}

func (a *Agent) scanAndDrop() error {
	// If we do not have the policy, fetch it from the blockchain. It is possible that a policy has not been set yet.
	// Keep attempting to fetch the policy until it is set. Once one is found, there is no need to keep fetching it,
	// as the retention policy is immutable.
	if a.policy == nil {
		policy, err := a.blockchainClient.GetRetentionPolicy()
		if err != nil {
			return err
		}

		a.policy = policy

		if a.policy == nil {
			a.logger.Info("Tried to run retention policy scan, but the retention policy has not been set yet")
			return nil
		} else {
			a.logger.Info("Retrieved retention policy from the blockchain")
			a.logger.Debug("Retrieved retention policy from the blockchain", zap.Any("policy", a.policy.Policy))
		}
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), a.config.EventRetention.ScanTimeout.Duration())
	defer cancelFunc()

	a.logger.Info("Scanning for and dropping events outside of the retention policy")
	if err := a.repository.Events().DropExpiredEvents(ctx, a.policy.Policy); err != nil {
		return err
	}

	a.logger.Info("Retention policy enforcement complete")
	return nil
}
