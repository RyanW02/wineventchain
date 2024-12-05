package swim

import (
	"context"
	"fmt"
	"github.com/RyanW02/wineventchain/offchain-interface/internal/config"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/transport"
	"github.com/hashicorp/memberlist"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"math"
	"math/rand"
	"sync"
	"time"
)

type SWIMTransport struct {
	config     config.Config
	logger     *zap.Logger
	cluster    *memberlist.Memberlist
	delegate   *delegate
	tx         chan []byte
	unicastTx  chan withErrorChan[unicastMsg]
	rx         chan inboundMessage
	shutdownCh chan chan error

	listeners  []transport.RxListener
	listenerMu sync.RWMutex
}

type withErrorChan[T any] struct {
	value T
	errCh chan error
}

type unicastMsg struct {
	target *memberlist.Node
	bytes  []byte
}

func newWithErrorChan[T any](value T) withErrorChan[T] {
	return withErrorChan[T]{
		value: value,
		errCh: make(chan error),
	}
}

func newUnicastMsg(target *memberlist.Node, bytes []byte) unicastMsg {
	return unicastMsg{
		target: target,
		bytes:  bytes,
	}
}

// Enforce interface constraints at compile time
var _ transport.EventTransport = (*SWIMTransport)(nil)

var (
	ErrTargetNotFound = errors.New("target not found")
	ErrClusterEmpty   = errors.New("cluster is empty")
)

func NewSWIMTransport(cfg config.Config, logger *zap.Logger) (*SWIMTransport, error) {
	rx := make(chan inboundMessage)

	var cluster *memberlist.Memberlist
	memberCount := func() int {
		if cluster == nil {
			return 0
		}

		return cluster.NumMembers()
	}

	delegate := newDelegate(logger, cfg.Transport.NodeName, rx, cfg.Transport.RetransmitMultiplier, memberCount)

	swimConfig, err := buildSWIMConfig(cfg, delegate)
	if err != nil {
		return nil, err
	}

	cluster, err = memberlist.Create(swimConfig)
	if err != nil {
		return nil, err
	}

	return &SWIMTransport{
		config:     cfg,
		logger:     logger,
		cluster:    cluster,
		delegate:   delegate,
		tx:         make(chan []byte),
		unicastTx:  make(chan withErrorChan[unicastMsg]),
		rx:         rx,
		shutdownCh: make(chan chan error),
		listeners:  make([]transport.RxListener, 0),
		listenerMu: sync.RWMutex{},
	}, nil
}

func (t *SWIMTransport) Shutdown() error {
	t.delegate.pruneShutdown <- struct{}{}

	resCh := make(chan error)
	t.shutdownCh <- resCh

	select {
	case err := <-resCh:
		return err
	case <-time.After(time.Second * 10):
		return fmt.Errorf("timed out waiting for shutdown")
	}
}

func (t *SWIMTransport) Identifier() string {
	return t.config.Transport.NodeName
}

func (t *SWIMTransport) AddRxListener(listener transport.RxListener) {
	t.listenerMu.Lock()
	defer t.listenerMu.Unlock()

	t.listeners = append(t.listeners, listener)
}

func (t *SWIMTransport) ClearListeners() {
	t.listenerMu.Lock()
	defer t.listenerMu.Unlock()

	t.listeners = make([]transport.RxListener, 0)
}

func (t *SWIMTransport) Broadcast(bytes []byte) error {
	t.tx <- bytes
	return nil
}

func (t *SWIMTransport) Unicast(ctx context.Context, targetIdentifier string, bytes []byte) error {
	for _, member := range t.cluster.Members() {
		if member.Name == targetIdentifier {
			return t.unicast(ctx, member, bytes)
		}
	}

	return ErrTargetNotFound
}

func (t *SWIMTransport) UnicastRandomNeighbour(ctx context.Context, bytes []byte) error {
	members := t.cluster.Members()
	if len(members) == 0 {
		return ErrClusterEmpty
	}

	// Exclude self
	for i, member := range members {
		if member.Name == t.config.Transport.NodeName {
			members = append(members[:i], members[i+1:]...)
			break
		}
	}

	neighbour := members[rand.Intn(len(members))]
	return t.unicast(ctx, neighbour, bytes)
}

func (t *SWIMTransport) unicast(ctx context.Context, target *memberlist.Node, bytes []byte) error {
	data := newWithErrorChan(newUnicastMsg(target, bytes))
	t.unicastTx <- data

	select {
	case err := <-data.errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *SWIMTransport) StartListener() {
	go t.delegate.StartPruneLoop()

	err := t.connect()
	if err != nil {
		t.logger.Error("failed to join cluster, will keep retrying", zap.Error(err))
	}

	connected := err == nil
	timer := time.NewTimer(time.Second * 10)
	defer timer.Stop()

	for {
		select {
		case bytes := <-t.tx:
			if !connected {
				t.logger.Warn("Got outbound SWIM transport message, but not connected to cluster yet. Dropping message.")
				continue
			}

			if t.config.Transport.UseGossip {
				t.delegate.Send(bytes)
			} else {
				go func() {
					for _, member := range t.cluster.Members() {
						// Do not send to self
						if member.Name == t.config.Transport.NodeName {
							continue
						}

						// Build a single, unlimited MTU frame
						fq := newFrameQueue(t.config.Transport.NodeName, bytes)
						frame, err := fq.Next(math.MaxUint)
						if err != nil {
							t.logger.Error("Failed to build frame for broadcast message", zap.Error(err))
							continue
						}

						ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
						if err := t.unicast(ctx, member, frame.Data); err != nil {
							t.logger.Error(
								"Failed to send unicast message",
								zap.Error(err),
								zap.String("destination", member.Name),
								zap.String("destination_addr", member.Addr.String()),
							)
						}
						cancel()
					}
				}()
			}

			continue
		case bytesWithChan := <-t.unicastTx:
			if !connected {
				t.logger.Warn("Got outbound SWIM transport message, but not connected to cluster yet. Dropping message.")
				continue
			}

			neighbour := bytesWithChan.value.target

			// SendReliable uses TCP, so there is not a need for manual packet fragmentation. Encode to a single frame.
			jumboFrame, err := newFrameQueue(t.config.Transport.NodeName, bytesWithChan.value.bytes).Next(math.MaxUint)
			if err != nil {
				t.logger.Error("Failed to build frame for unicast message", zap.Error(err))
				bytesWithChan.errCh <- err
				continue
			}

			encoded, err := jumboFrame.Marshal()
			if err != nil {
				t.logger.Error("Failed to encode unicast message", zap.Error(err))
				bytesWithChan.errCh <- err
			}

			err = t.cluster.SendReliable(neighbour, encoded)
			bytesWithChan.errCh <- err
			if err != nil {
				t.logger.Error(
					"Failed to send unicast message",
					zap.Error(err),
					zap.String("destination", neighbour.Name),
					zap.String("destination_addr", neighbour.Addr.String()),
				)
				continue
			}
		case msg := <-t.rx:
			t.listenerMu.RLock()
			for _, listener := range t.listeners {
				go listener(msg.source, msg.data)
			}
			t.listenerMu.RUnlock()
		case errCh := <-t.shutdownCh:
			t.logger.Info("Shutting down SWIM transport")

			if err := t.cluster.Leave(time.Second * 5); err != nil {
				t.logger.Error("failed to leave cluster", zap.Error(err))
				errCh <- err
			}

			t.logger.Debug("Left cluster")

			if err := t.cluster.Shutdown(); err != nil {
				t.logger.Error("failed to shutdown cluster", zap.Error(err))
				errCh <- err
			}

			t.delegate.StopPruneLoop()
			errCh <- nil
		case <-timer.C:
			if !connected {
				if err := t.connect(); err != nil {
					t.logger.Error("failed to join cluster, will keep retrying", zap.Error(err))
					continue
				}

				connected = true
			}
		}
	}
}

func (t *SWIMTransport) connect() error {
	peerCount, err := t.cluster.Join(t.config.Transport.Peers)
	if err != nil {
		return err
	}

	t.logger.Info("Joined cluster", zap.Int("peer_count", peerCount))
	return nil
}

func buildSWIMConfig(appConfig config.Config, delegate *delegate) (*memberlist.Config, error) {
	var conf *memberlist.Config

	switch appConfig.Transport.NetworkType.ConvertCase() {
	case config.NetworkTypeWAN:
		conf = memberlist.DefaultWANConfig()
	case config.NetworkTypeLAN:
		conf = memberlist.DefaultLANConfig()
	case config.NetworkTypeLocal:
		conf = memberlist.DefaultLocalConfig()
	default:
		return nil, fmt.Errorf("invalid network type: %s", appConfig.Transport.NetworkType)
	}

	conf.Name = appConfig.Transport.NodeName
	conf.BindAddr = appConfig.Transport.BindAddress
	conf.BindPort = appConfig.Transport.BindPort

	conf.RetransmitMult = appConfig.Transport.RetransmitMultiplier
	conf.GossipNodes = appConfig.Transport.RetransmitMultiplier

	if appConfig.Transport.UseEncryption {
		conf.SecretKey = []byte(appConfig.Transport.SharedKey)
	}

	conf.Delegate = delegate

	return conf, nil
}
