package pool

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type connection struct {
	id int
}

var (
	passingTest TestFunc[connection] = func(c connection) bool {
		return true
	}

	failingTest TestFunc[connection] = func(c connection) bool {
		return false
	}
)

func TestTakeConns(t *testing.T) {
	p := NewPool[connection](nil, PoolConfig[connection]{
		LivenessValidThreshold: 15,
		TestFunc:               passingTest,
	})

	c1 := connection{}
	c2 := connection{}
	c3 := connection{}

	p.Add(c1, c2, c3)

	t1, err := p.Get()
	require.NoError(t, err)

	t2, err := p.Get()
	require.NoError(t, err)

	t3, err := p.Get()
	require.NoError(t, err)

	require.Equal(t, &c1, t1)
	require.Equal(t, &c2, t2)
	require.Equal(t, &c3, t3)
}

func TestRoundRobin(t *testing.T) {
	p := NewPool[connection](nil, PoolConfig[connection]{
		LivenessValidThreshold: 15,
		TestFunc:               passingTest,
	})

	c1 := connection{}
	c2 := connection{}

	p.Add(c1, c2)

	t1, err := p.Get()
	require.NoError(t, err)

	t2, err := p.Get()
	require.NoError(t, err)

	t3, err := p.Get()
	require.NoError(t, err)

	t4, err := p.Get()
	require.NoError(t, err)

	t5, err := p.Get()
	require.NoError(t, err)

	require.Equal(t, &c1, t1)
	require.Equal(t, &c2, t2)
	require.Equal(t, &c1, t3)
	require.Equal(t, &c2, t4)
	require.Equal(t, &c1, t5)
}

func TestFailing(t *testing.T) {
	p := NewPool[connection](nil, PoolConfig[connection]{
		LivenessValidThreshold: 15,
		TestFunc:               failingTest,
	})

	c1 := connection{}
	c2 := connection{}

	p.Add(c1, c2)

	_, err := p.Get()
	require.ErrorIs(t, err, ErrPoolEmpty)

	_, err = p.Get()
	require.Error(t, err, ErrPoolEmpty)
}

func TestLivenessTimeout(t *testing.T) {
	alive := true
	testFunc := func(connection) bool {
		return alive
	}

	p := NewPool[connection](nil, PoolConfig[connection]{
		LivenessValidThreshold: time.Millisecond * 100,
		TestFunc:               testFunc,
	})

	c1 := connection{}
	c2 := connection{}

	p.Add(c1, c2)

	t1, err := p.Get()
	require.NoError(t, err)
	require.Equal(t, &c1, t1)

	t2, err := p.Get()
	require.NoError(t, err)
	require.Equal(t, &c2, t2)

	alive = false

	t3, err := p.Get()
	require.NoError(t, err)
	require.Equal(t, &c1, t3)

	t4, err := p.Get()
	require.NoError(t, err)
	require.Equal(t, &c2, t4)

	time.Sleep(time.Millisecond * 150)

	_, err = p.Get()
	require.ErrorIs(t, err, ErrPoolEmpty)
}

func TestDeadConnFixed(t *testing.T) {
	alive := true
	testFunc := func(connection) bool {
		return alive
	}

	p := NewPool[connection](nil, PoolConfig[connection]{
		LivenessValidThreshold: 0,
		TestFunc:               testFunc,
		DeadConnCheckInterval:  time.Millisecond * 20,
	})

	c1 := connection{}
	c2 := connection{}

	p.Add(c1, c2)

	t1, err := p.Get()
	require.NoError(t, err)
	require.Equal(t, &c1, t1)

	alive = false
	_, err = p.Get()
	require.ErrorIs(t, err, ErrPoolEmpty)

	// Wait for dead connection checker to run
	alive = true
	time.Sleep(time.Millisecond * 40)

	t2, err := p.Get()
	require.NoError(t, err)
	require.Equal(t, &c1, t2)

	t3, err := p.Get()
	require.NoError(t, err)
	require.Equal(t, &c2, t3)
}

func TestOneDead(t *testing.T) {
	testFunc := func(c connection) bool {
		return c.id != 1
	}

	p := NewPool[connection](nil, PoolConfig[connection]{
		LivenessValidThreshold: 0,
		TestFunc:               testFunc,
		DeadConnCheckInterval:  time.Millisecond * 20,
	})

	c1 := connection{1}
	c2 := connection{2}

	p.Add(c1, c2)

	for i := 0; i < 10; i++ {
		got, err := p.Get()
		require.NoError(t, err)
		require.Equal(t, &c2, got)
	}
}
