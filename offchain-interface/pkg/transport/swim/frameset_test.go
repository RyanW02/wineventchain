package swim

import (
	"bytes"
	"errors"
	"github.com/stretchr/testify/require"
	"io"
	"testing"
)

func TestUncompressed(t *testing.T) {
	fq := newFrameQueue("source", []byte("test"))
	frames := allFrames(t, 1280, fq)

	require.Equal(t, 1, len(frames))
	require.Equal(t, false, frames[0].IsCompressed)
	require.Equal(t, uint16(0), frames[0].FrameNumber)
	require.Equal(t, true, frames[0].IsLast)

	decoder := newFrameQueueDecoder(fq.id)
	require.NoError(t, decoder.ReadFrame(frames[0]))

	require.True(t, decoder.ReceivedAll())

	data, err := decoder.Decode()
	require.NoError(t, err)

	require.Equal(t, []byte("test"), data)
}

func TestCompressed(t *testing.T) {
	// Easily compressible data
	data := bytes.Repeat([]byte("a"), 1281)

	fq := newFrameQueue("source", data)
	frames := allFrames(t, 1280, fq)

	require.Equal(t, 1, len(frames))
	require.Equal(t, true, frames[0].IsCompressed)

	decoder := newFrameQueueDecoder(fq.id)
	require.NoError(t, decoder.ReadFrame(frames[0]))

	require.True(t, decoder.ReceivedAll())

	decoded, err := decoder.Decode()
	require.NoError(t, err)

	require.Equal(t, data, decoded)
}

func TestCompressedMultipleFrames(t *testing.T) {
	// Data that is hard to compress
	data := []byte("be01a140ac0e6f560b1f0e4a983401598a0c36JK!JK123njknjk!UI2io3m,qdsdgh';#[")
	source := "source123"

	fq := newFrameQueue(source, data)
	frames := allFrames(t, mtuForDataSize(30, source), fq)

	require.Equal(t, 3, len(frames))
	require.Equal(t, true, frames[0].IsCompressed)
	require.Equal(t, true, frames[1].IsCompressed)
	require.Equal(t, true, frames[2].IsCompressed)

	decoder := newFrameQueueDecoder(fq.id)
	require.NoError(t, decoder.ReadFrame(frames[0]))
	require.NoError(t, decoder.ReadFrame(frames[1]))
	require.NoError(t, decoder.ReadFrame(frames[2]))

	require.True(t, decoder.ReceivedAll())

	decoded, err := decoder.Decode()
	require.NoError(t, err)

	require.Equal(t, data, decoded)
}

func TestDecodeDuplicateFrames(t *testing.T) {
	// Data that is hard to compress
	data := []byte("be01a140ac0e6f560b1f0e4a983401598a0c36JK!JK123njknjk!UI2io3m,qdsdgh';#[")
	source := "source123"

	fq := newFrameQueue(source, data)
	frames := allFrames(t, mtuForDataSize(30, source), fq)

	require.Equal(t, 3, len(frames))
	require.Equal(t, true, frames[0].IsCompressed)
	require.Equal(t, true, frames[1].IsCompressed)
	require.Equal(t, true, frames[2].IsCompressed)

	decoder := newFrameQueueDecoder(fq.id)
	require.False(t, decoder.ReceivedAll())
	require.NoError(t, decoder.ReadFrame(frames[0]))
	require.False(t, decoder.ReceivedAll())
	require.NoError(t, decoder.ReadFrame(frames[0]))
	require.False(t, decoder.ReceivedAll())
	require.NoError(t, decoder.ReadFrame(frames[1]))
	require.False(t, decoder.ReceivedAll())
	require.NoError(t, decoder.ReadFrame(frames[1]))
	require.False(t, decoder.ReceivedAll())
	require.NoError(t, decoder.ReadFrame(frames[2]))
	require.NoError(t, decoder.ReadFrame(frames[2]))

	require.True(t, decoder.ReceivedAll())

	decoded, err := decoder.Decode()
	require.NoError(t, err)

	require.Equal(t, data, decoded)
}

func TestDecodeRandomOrder(t *testing.T) {
	// Data that is hard to compress
	data := []byte("be01a140ac0e6f560b1f0e4a983401598a0c36JK!JK123njknjk!UI2io3m,qdsdgh';#[")
	source := "source123"

	fq := newFrameQueue(source, data)
	frames := allFrames(t, mtuForDataSize(30, source), fq)

	require.Equal(t, 3, len(frames))
	require.Equal(t, true, frames[0].IsCompressed)
	require.Equal(t, true, frames[1].IsCompressed)
	require.Equal(t, true, frames[2].IsCompressed)

	decoder := newFrameQueueDecoder(fq.id)
	require.False(t, decoder.ReceivedAll())

	require.NoError(t, decoder.ReadFrame(frames[2]))
	require.False(t, decoder.ReceivedAll())
	require.NoError(t, decoder.ReadFrame(frames[2]))
	require.False(t, decoder.ReceivedAll())

	require.NoError(t, decoder.ReadFrame(frames[0]))
	require.False(t, decoder.ReceivedAll())
	require.NoError(t, decoder.ReadFrame(frames[0]))
	require.False(t, decoder.ReceivedAll())

	require.NoError(t, decoder.ReadFrame(frames[1]))
	require.True(t, decoder.ReceivedAll())

	decoded, err := decoder.Decode()
	require.NoError(t, err)

	require.Equal(t, data, decoded)
}

func TestMaxFrames(t *testing.T) {
	// Data that is hard to compress
	data := []byte("be01a140ac0e6f560b1f0e4a98340158a0c36JKbe01a140ac0e6f560b1f0e4a98340158a0c36JK!JK123njknjk!UI2io3m,qdsdgh';#[")
	source := "source"

	mtu := mtuForDataSize(1, source)

	fq := newFrameQueue(source, data)
	_, err := allFramesWithError(mtu, fq)
	require.Equal(t, ErrMaxFrameCountExceeded, err)
}

func allFrames(t *testing.T, mtu uint, fq *frameQueue) []*frame {
	t.Helper()

	frames, err := allFramesWithError(mtu, fq)
	require.NoError(t, err)

	return frames
}

func allFramesWithError(mtu uint, fq *frameQueue) ([]*frame, error) {
	var frames []*frame
	for {
		frame, err := fq.Next(mtu)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			} else {
				return nil, err
			}
		}

		frames = append(frames, frame)
	}

	return frames, nil
}

func mtuForDataSize(dataSize uint, source string) uint {
	return 1 + 16 + 2 + 1 + 2 + uint(len([]byte(source))) + 4 + dataSize + 4
}
