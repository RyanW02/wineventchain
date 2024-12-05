package broadcast

import "sync"

type EmptyBroadcastChannel = BroadcastChannel[struct{}]

type BroadcastChannel[T any] struct {
	mu        sync.RWMutex
	listeners []chan T
}

func NewBroadcastChannel[T any]() *BroadcastChannel[T] {
	return &BroadcastChannel[T]{
		listeners: make([]chan T, 0),
	}
}

func NewEmptyBroadcastChannel() *EmptyBroadcastChannel {
	return NewBroadcastChannel[struct{}]()
}

func (b *BroadcastChannel[T]) Subscribe() chan T {
	ch := make(chan T)

	b.mu.Lock()
	defer b.mu.Unlock()

	b.listeners = append(b.listeners, ch)
	return ch
}

func (b *BroadcastChannel[T]) Publish(value T) {
	go func() {
		b.mu.RLock()
		defer b.mu.RUnlock()

		for _, listener := range b.listeners {
			listener <- value
		}
	}()
}

func (b *BroadcastChannel[T]) CloseAll() {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, listener := range b.listeners {
		close(listener)
	}
	b.listeners = make([]chan T, 0)
}
