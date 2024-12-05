package swim

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSimple(t *testing.T) {
	f := newFrame(false, uuid.New(), 1, true, "source", []byte("test"))
	bytes, err := f.Marshal()
	require.NoError(t, err)

	var unmarshalled frame
	require.NoError(t, unmarshalled.Unmarshal(bytes))

	require.Equal(t, f, &unmarshalled)
}

func TestEmptySourceName(t *testing.T) {
	f := newFrame(false, uuid.New(), 1, true, "", []byte("test"))
	bytes, err := f.Marshal()
	require.NoError(t, err)

	var unmarshalled frame
	require.NoError(t, unmarshalled.Unmarshal(bytes))

	require.Equal(t, f, &unmarshalled)
}

func TestEmptyData(t *testing.T) {
	f := newFrame(false, uuid.New(), 1, true, "source", []byte(""))
	bytes, err := f.Marshal()
	require.NoError(t, err)

	var unmarshalled frame
	require.NoError(t, unmarshalled.Unmarshal(bytes))

	require.Equal(t, f, &unmarshalled)
}

func TestNilData(t *testing.T) {
	f := newFrame(false, uuid.New(), 1, true, "source", nil)
	bytes, err := f.Marshal()
	require.NoError(t, err)

	var unmarshalled frame
	require.NoError(t, unmarshalled.Unmarshal(bytes))

	// Unmarshals as []uint8{}, not nil so comparison fails
	require.Equal(t, f.IsCompressed, unmarshalled.IsCompressed)
	require.Equal(t, f.FrameNumber, unmarshalled.FrameNumber)
	require.Equal(t, f.IsLast, unmarshalled.IsLast)
	require.Equal(t, f.Source, unmarshalled.Source)
	require.Equal(t, []byte{}, unmarshalled.Data)
}

func TestEmptySourceAndData(t *testing.T) {
	f := newFrame(false, uuid.New(), 1, true, "", []byte(""))
	bytes, err := f.Marshal()
	require.NoError(t, err)

	var unmarshalled frame
	require.NoError(t, unmarshalled.Unmarshal(bytes))

	require.Equal(t, f, &unmarshalled)
}
