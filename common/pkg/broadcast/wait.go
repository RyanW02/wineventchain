package broadcast

import (
	"time"
)

type WaitBroadcastChannel[T any] struct {
	bc *BroadcastChannel[chan T]
}

func NewWaitBroadcastChannel[T any]() *WaitBroadcastChannel[T] {
	return &WaitBroadcastChannel[T]{
		bc: NewBroadcastChannel[chan T](),
	}
}

func (w *WaitBroadcastChannel[T]) Subscribe() chan chan T {
	return w.bc.Subscribe()
}

// PublishAndWait returns each value provided by the listeners, and a boolean indicating whether the operation timed
// out waiting for responses.
func (w *WaitBroadcastChannel[T]) PublishAndWait(timeout time.Duration) ([]T, bool) {
	ch := make(chan T)

	w.bc.mu.RLock()
	defer w.bc.mu.RUnlock()

	if len(w.bc.listeners) == 0 {
		return nil, false
	}

	// Publish
	go func() {
		for _, listener := range w.bc.listeners {
			listener <- ch
		}
	}()

	timer := time.After(timeout)
	results := make([]T, 0, len(w.bc.listeners))
	for {
		select {
		case v := <-ch:
			results = append(results, v)
			if len(results) == len(w.bc.listeners) {
				return results, false
			}
		case <-timer:
			return results, true
		}
	}
}
