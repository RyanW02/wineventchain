package swim

import (
	"bytes"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"sync"
)

type frameQueueDecoder struct {
	mu            sync.RWMutex
	id            uuid.UUID
	receivedCount uint16
	terminated    bool
	isCompressed  bool
	frames        []*frame
}

var (
	ErrFrameStreamUuidMismatch = errors.New("frame set UUID mismatch")
	ErrNotTerminated           = errors.New("frame set not terminated")
	ErrNoFrames                = errors.New("no frames received")
)

func newFrameQueueDecoder(id uuid.UUID) *frameQueueDecoder {
	return &frameQueueDecoder{
		mu:            sync.RWMutex{},
		id:            id,
		receivedCount: 0,
	}
}

func (d *frameQueueDecoder) ReadBytes(bytes []byte) error {
	var f frame
	if err := f.Unmarshal(bytes); err != nil {
		return err
	}

	return d.ReadFrame(&f)
}

func (d *frameQueueDecoder) ReadFrame(f *frame) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.receivedCount > MaxFrameCount || f.FrameNumber > MaxFrameCount {
		return ErrMaxFrameCountExceeded
	}

	if d.id != f.StreamId {
		return errors.Wrapf(ErrFrameStreamUuidMismatch, "expected %s, got %s", d.id, f.StreamId)
	}

	// If first frame received, set compression flag
	if d.receivedCount == 0 {
		d.isCompressed = f.IsCompressed
	} else { // Otherwise, compare compression flags
		if d.isCompressed != f.IsCompressed {
			return ErrMixedCompression
		}
	}

	// Ignore duplicate frames
	if len(d.frames) >= int(f.FrameNumber+1) && d.frames[f.FrameNumber] != nil {
		return nil
	}

	// Extend the slice if necessary
	if int(f.FrameNumber) >= len(d.frames) {
		newFrames := make([]*frame, f.FrameNumber+1)
		copy(newFrames, d.frames)
		d.frames = newFrames
	}

	// Store the frame
	d.frames[f.FrameNumber] = f
	d.receivedCount++

	if f.IsLast {
		d.terminated = true
	}

	return nil
}

func (d *frameQueueDecoder) HasReceivedFrame(frameNumber uint16) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, frame := range d.frames {
		if frame.FrameNumber == frameNumber {
			return true
		}
	}

	return false
}

func (d *frameQueueDecoder) ReceivedAll() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.terminated && d.receivedCount == uint16(len(d.frames))
}

func (d *frameQueueDecoder) SourceName() (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.frames) == 0 {
		return "", ErrNoFrames
	}

	return d.frames[0].Source, nil
}

func (d *frameQueueDecoder) Decode() ([]byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.terminated {
		return nil, ErrNotTerminated
	}

	var buf bytes.Buffer
	for _, f := range d.frames {
		buf.Write(f.Data)
	}

	if d.isCompressed {
		return decompress(buf.Bytes())
	} else {
		return buf.Bytes(), nil
	}
}
