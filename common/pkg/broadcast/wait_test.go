package broadcast

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestWaitSimple(t *testing.T) {
	ch := NewWaitBroadcastChannel[int]()

	for i := 0; i < 10; i++ {
		go func(i int, sub chan chan int) {
			res := <-sub
			res <- i
		}(i, ch.Subscribe())
	}

	results, timedOut := ch.PublishAndWait(time.Second)
	require.False(t, timedOut)
	require.Len(t, results, 10)

	for i := 0; i < 10; i++ {
		require.Contains(t, results, i)
	}
}

func TestWaitNoSubscribers(t *testing.T) {
	ch := NewWaitBroadcastChannel[int]()

	results, timedOut := ch.PublishAndWait(time.Second)
	require.False(t, timedOut)
	require.Empty(t, results)
}

func TestWaitTimeout(t *testing.T) {
	ch := NewWaitBroadcastChannel[int]()

	for i := 0; i < 10; i++ {
		go func(i int, sub chan chan int) {
			res := <-sub
			time.Sleep(time.Second * 2)
			res <- i
		}(i, ch.Subscribe())
	}

	_, timedOut := ch.PublishAndWait(time.Second)
	require.True(t, timedOut)
}
