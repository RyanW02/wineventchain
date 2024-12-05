package pool

import (
	"context"
	"errors"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

type TestFunc[T any] func(T) bool
type DestructorFunc[T any] func(T) error

// Pool is a round-robin pool for load-balancing connections. Each connection is not unique: the same connection
// may be held by multiple clients. **Therefore, the connection object must be thread-safe**.
type Pool[T comparable] struct {
	mu                     sync.Mutex
	idx                    int
	conns                  []T
	deadConns              []T
	lastTestTime           map[T]time.Time
	livenessValidThreshold time.Duration
	testFunc               TestFunc[T]
	destructorFunc         DestructorFunc[T]
	deadConnCheckInterval  time.Duration
	closeCh                chan struct{}
}

type PoolConfig[T any] struct {
	LivenessValidThreshold time.Duration
	DeadConnCheckInterval  time.Duration
	TestFunc               TestFunc[T]
	DestructorFunc         DestructorFunc[T]
}

// NewPool creates a new pool with the given configuration.
func NewPool[T comparable](conns []T, config PoolConfig[T]) *Pool[T] {
	if config.TestFunc == nil {
		config.TestFunc = func(T) bool { return true }
	}

	pool := Pool[T]{
		mu:                     sync.Mutex{},
		idx:                    0,
		conns:                  make([]T, 0),
		deadConns:              make([]T, 0),
		lastTestTime:           make(map[T]time.Time),
		livenessValidThreshold: config.LivenessValidThreshold,
		testFunc:               config.TestFunc,
		destructorFunc:         config.DestructorFunc,
	}

	if len(conns) > 0 {
		pool.Add(conns...)
	}

	if config.DeadConnCheckInterval > 0 {
		pool.deadConnCheckInterval = config.DeadConnCheckInterval
		pool.closeCh = make(chan struct{})
		go pool.startDeadConnTester()
	}

	return &pool
}

var ErrPoolEmpty = errors.New("pool is empty")

func (p *Pool[T]) Add(conn ...T) {
	alive := make([]T, 0, len(conn)) // Assume all will be alive when allocating slice
	dead := make([]T, 0)             // Assume no dead when allocating slice

	for _, c := range conn {
		if p.testFunc(c) {
			alive = append(alive, c)
		} else {
			dead = append(dead, c)
		}

		p.mu.Lock()
		p.lastTestTime[c] = time.Now()
		p.mu.Unlock()
	}

	p.mu.Lock()
	p.conns = append(p.conns, alive...)
	p.deadConns = append(p.deadConns, dead...)
	p.mu.Unlock()
}

func (p *Pool[T]) Remove(conn T) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, c := range p.conns {
		if c == conn {
			p.conns = append(p.conns[:i], p.conns[i+1:]...)
			return
		}
	}

	for i, c := range p.deadConns {
		if c == conn {
			p.deadConns = append(p.deadConns[:i], p.deadConns[i+1:]...)
			return
		}
	}
}

func (p *Pool[T]) Get() (*T, error) {
	p.mu.Lock()

	if len(p.conns) == 0 {
		p.mu.Unlock()
		return nil, ErrPoolEmpty
	}

	p.idx += 1
	if p.idx >= len(p.conns) {
		p.idx = 0
	}

	conn := p.conns[p.idx]

	// Check if we need to test the connection
	lastTested, ok := p.lastTestTime[conn]
	if ok && time.Since(lastTested) <= p.livenessValidThreshold {
		p.mu.Unlock()
		return &conn, nil
	}

	p.mu.Unlock()

	if p.testFunc(conn) {
		return &conn, nil
	} else {
		// If connection is dead, remove it from the pool and try again
		p.mu.Lock()
		p.conns = append(p.conns[:p.idx], p.conns[p.idx+1:]...)
		p.deadConns = append(p.deadConns, conn)
		p.mu.Unlock()
		return p.Get()
	}
}

func (p *Pool[T]) GetAll(includeDead bool) []T {
	p.mu.Lock()

	// If the client wants dead connections as well, just return all connections now
	if includeDead {
		conns := make([]T, 0, len(p.conns)+len(p.deadConns))
		for i := range p.conns {
			conns = append(conns, p.conns[i])
		}

		for i := range p.deadConns {
			conns = append(conns, p.deadConns[i])
		}

		p.mu.Unlock()
		return conns
	}

	var conns []T
	for _, conn := range p.conns {
		// Check if we need to test the connection
		lastTested, ok := p.lastTestTime[conn]
		if ok && time.Since(lastTested) <= p.livenessValidThreshold {
			conns = append(conns, conn)
		}

		p.mu.Unlock()
		isAlive := p.testFunc(conn)
		p.mu.Lock()

		if isAlive {
			conns = append(conns, conn)
		} else {
			// If connection is dead, remove it from the pool
			p.conns = append(p.conns[:p.idx], p.conns[p.idx+1:]...)
			p.deadConns = append(p.deadConns, conn)
		}
	}

	p.mu.Unlock()
	return conns
}

func (p *Pool[T]) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closeCh != nil {
		close(p.closeCh)
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFunc()

	group, _ := errgroup.WithContext(ctx)
	for _, conn := range p.conns {
		conn := conn

		group.Go(func() error {
			return p.destructorFunc(conn)
		})
	}

	for _, conn := range p.deadConns {
		conn := conn

		group.Go(func() error {
			return p.destructorFunc(conn)
		})
	}

	return group.Wait()
}

func (p *Pool[T]) startDeadConnTester() {
	ticker := time.NewTicker(p.deadConnCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			for i := len(p.deadConns) - 1; i >= 0; i-- {
				if p.testFunc(p.deadConns[i]) {
					p.mu.Lock()
					p.conns = append(p.conns, p.deadConns[i])
					p.deadConns = append(p.deadConns[:i], p.deadConns[i+1:]...)
					p.mu.Unlock()
				}
			}
		case <-p.closeCh:
			return
		}
	}
}
