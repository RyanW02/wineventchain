package blockchain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/RyanW02/wineventchain/common/pkg/pool"
	"github.com/RyanW02/wineventchain/common/pkg/proof"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	"github.com/RyanW02/wineventchain/common/pkg/types/retention"
	"github.com/RyanW02/wineventchain/common/pkg/types/rpc"
	"github.com/RyanW02/wineventchain/offchain-interface/internal/config"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/state"
	"github.com/cometbft/cometbft/crypto/merkle"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	"github.com/cometbft/cometbft/rpc/client/http"
	rpctypes "github.com/cometbft/cometbft/rpc/jsonrpc/types"
	"go.uber.org/zap"
	"strings"
	"time"
)

type RoundRobinClient struct {
	config       config.Config
	logger       *zap.Logger
	pool         *pool.Pool[http.HTTP]
	proofRuntime *merkle.ProofRuntime
}

var (
	ErrABCIQueryFailed = fmt.Errorf("ABCI query failed")
	ErrEventNotFound   = errors.New("tx not found")

	ErrNotEnoughNodes = errors.New("not enough nodes to satisfy minimum nodes requirement")
	ErrPolicyMismatch = errors.New("policies do not match")
)

func NewRoundRobinClient(config config.Config, logger *zap.Logger, clients []http.HTTP) *RoundRobinClient {
	return &RoundRobinClient{
		config: config,
		logger: logger,
		pool: pool.NewPool[http.HTTP](clients, pool.PoolConfig[http.HTTP]{
			LivenessValidThreshold: 10 * time.Second,
			DeadConnCheckInterval:  time.Second * 15,
			TestFunc: func(c http.HTTP) bool {
				ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancelFunc()

				_, err := c.ABCIInfo(ctx)
				return err == nil
			},
			DestructorFunc: http.HTTP.Stop,
		}),
	}
}

func (c *RoundRobinClient) Close() {
	c.pool.Close()
}

func (c *RoundRobinClient) GetEventByTx(txHash []byte) (events.EventWithMetadata, error) {
	metadata, err := c.GetEventMetadataByTx(txHash)
	if err != nil {
		return events.EventWithMetadata{}, err
	}

	return c.GetEventById(metadata.EventId)
}

func (c *RoundRobinClient) GetEventMetadataByTx(txHash []byte) (events.Metadata, error) {
	conn, err := c.pool.Get()
	if err != nil {
		return events.Metadata{}, err
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()

	tx, err := conn.Tx(ctx, txHash, false)
	if err != nil {
		var rpcError *rpctypes.RPCError
		if errors.As(err, &rpcError) && strings.Contains(rpcError.Data, "not found") {
			return events.Metadata{}, ErrEventNotFound
		}

		return events.Metadata{}, err
	}

	var res events.CreateResponse
	if err := json.Unmarshal(tx.TxResult.Data, &res); err != nil {
		return events.Metadata{}, err
	}

	return res.Metadata, nil
}

func (c *RoundRobinClient) GetEventById(eventId events.EventHash) (events.EventWithMetadata, error) {
	conn, err := c.pool.Get()
	if err != nil {
		return events.EventWithMetadata{}, err
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()

	// Create data payload to route to the correct sub-app
	data := rpc.MuxedRequest{App: events.AppName}
	dataMarshalled, err := json.Marshal(data)
	if err != nil {
		return events.EventWithMetadata{}, err
	}

	path := fmt.Sprintf("/event-by-id/%s", eventId.String())
	res, err := conn.ABCIQueryWithOptions(ctx, path, dataMarshalled, ABCIQueryOptions)
	if err != nil {
		return events.EventWithMetadata{}, err
	}

	if res.Response.Code != 0 {
		if res.Response.Codespace == events.Codespace && res.Response.Code == events.CodeEventNotFound {
			return events.EventWithMetadata{}, ErrEventNotFound
		}

		return events.EventWithMetadata{}, fmt.Errorf(
			"%w, code: %s:%d, log: %s, info: %s",
			ErrABCIQueryFailed, res.Response.Codespace, res.Response.Code, res.Response.Log, res.Response.Info,
		)
	}

	var event events.EventWithMetadata
	if err := json.Unmarshal(res.Response.Value, &event); err != nil {
		return events.EventWithMetadata{}, err
	}

	// Validate proof
	if err := proof.ValidateProofOps(res.Response.ProofOps); err != nil {
		return events.EventWithMetadata{}, err
	}

	return event, nil
}

// SearchEvents searches for events between the given block heights (inclusive of lowerHeight, exclusive of upperHeight)
func (c *RoundRobinClient) SearchEvents(lowerHeight, upperHeight int64, page, limit int) ([]state.MissingEvent, int, error) {
	conn, err := c.pool.Get()
	if err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf("tx.height >= %d AND tx.height < %d AND %s.%s='%s'",
		lowerHeight, upperHeight, events.EventCreate, events.AttributeType, events.AttributeValueCreate,
	)

	ctx, cancelFunc := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelFunc()

	res, err := conn.TxSearch(ctx, query, true, &page, &limit, "asc")
	if err != nil {
		return nil, 0, err
	}
	cancelFunc()

	missingEvents := make([]state.MissingEvent, 0, len(res.Txs))
	for _, tx := range res.Txs {
		var decoded events.CreateResponse
		if err := json.Unmarshal(tx.TxResult.Data, &decoded); err != nil {
			c.logger.Error("Failed to unmarshal tx result data", zap.Error(err), zap.ByteString("tx_hash", tx.Hash))
			continue // Attempt to return all the transactions we can
		}

		missingEvents = append(missingEvents, state.MissingEvent{
			EventId:      decoded.Metadata.EventId,
			ReceivedTime: time.Now(),
			BlockHeight:  tx.Height,
		})
	}

	return missingEvents, res.TotalCount, nil
}

// GetRetentionPolicy fetches the retention policy from all available blockchain nodes and compares them, to ensure
// that a malicious node is not returning a different policy to others.
func (c *RoundRobinClient) GetRetentionPolicy() (*offchain.StoredPolicy, error) {
	clients := c.pool.GetAll(false)

	if len(clients) < c.config.Blockchain.MinimumNodes {
		return nil, fmt.Errorf("%w, expected: %d, actual: %d", ErrNotEnoughNodes, c.config.Blockchain.MinimumNodes, len(clients))
	}

	var errs []error
	policies := make([]*offchain.StoredPolicy, 0, len(clients))
	for _, client := range clients {
		// Don't use defer cancelFunc in loop
		ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
		policy, err := c.getRetentionPolicyForClient(ctx, client)
		cancelFunc()
		if err != nil {
			c.logger.Error("Failed to get retention policy", zap.Error(err), zap.String("client", client.Remote()))
			errs = append(errs, err)
			continue
		}

		policies = append(policies, policy)
	}

	if len(policies) < c.config.Blockchain.MinimumNodes || len(policies) == 0 {
		return nil, errors.Join(
			append(errs,
				fmt.Errorf("%w, expected: %d, actual: %d", ErrNotEnoughNodes, c.config.Blockchain.MinimumNodes, len(policies)),
			)...,
		)
	}

	// Ensure all policies are the same
	if len(policies) == 1 {
		return policies[0], nil
	}

	firstPolicy := policies[0]
	for i := 1; i < len(policies); i++ {
		policy := policies[i]

		if policy == nil && firstPolicy == nil {
			continue
		} else if policy == nil || firstPolicy == nil {
			return nil, fmt.Errorf("%w, policy[0]: %v\npolicy[%d]: %v", ErrPolicyMismatch, firstPolicy, i, policy)
		} else {
			if !firstPolicy.Equal(*policy) {
				return nil, fmt.Errorf("%w, policy[0]: %v\npolicy[%d]: %v", ErrPolicyMismatch, firstPolicy, i, policy)
			}
		}
	}

	return firstPolicy, nil
}

func (c *RoundRobinClient) getRetentionPolicyForClient(ctx context.Context, client http.HTTP) (*offchain.StoredPolicy, error) {
	options := rpcclient.ABCIQueryOptions{
		Height: 0,
		Prove:  false,
	}

	data := rpc.MuxedRequest{App: retention.AppName}
	marshalled, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	res, err := client.ABCIQueryWithOptions(ctx, "/", marshalled, options)
	if err != nil {
		return nil, err
	}

	if res.Response.Codespace == retention.Codespace && res.Response.Code == retention.CodeOk {
		var policy offchain.StoredPolicy
		if err := json.Unmarshal(res.Response.Value, &policy); err != nil {
			return nil, err
		}

		return &policy, nil
	} else if res.Response.Codespace == retention.Codespace && res.Response.Code == retention.CodePolicyNotSet {
		return nil, nil
	} else {
		return nil, fmt.Errorf("%w, code: %s:%d, log: %s", ErrABCIQueryFailed, res.Response.Codespace, res.Response.Code, res.Response.Log)
	}
}
