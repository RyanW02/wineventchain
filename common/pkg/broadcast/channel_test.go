package broadcast

import (
	"github.com/RyanW02/wineventchain/common/pkg/utils"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSimple(t *testing.T) {
	ch := NewBroadcastChannel[int]()

	sub1 := ch.Subscribe()
	sub2 := ch.Subscribe()

	ch.Publish(1)

	if <-sub1 != 1 {
		t.Error("Expected 1")
	}

	if <-sub2 != 1 {
		t.Error("Expected 1")
	}
}

func TestEmpty(t *testing.T) {
	ch := NewBroadcastChannel[int]()
	ch.Publish(2)
}

func TestClose(t *testing.T) {
	ch := NewBroadcastChannel[int]()

	sub1 := ch.Subscribe()
	sub2 := ch.Subscribe()

	require.False(t, utils.IsClosed(sub1))
	require.False(t, utils.IsClosed(sub2))

	ch.CloseAll()

	require.True(t, utils.IsClosed(sub1))
	require.True(t, utils.IsClosed(sub2))
}
