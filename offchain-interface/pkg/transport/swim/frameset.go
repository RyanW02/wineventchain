package swim

import (
	"bytes"
	"compress/zlib"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"io"
	"sync"
)

type frameQueue struct {
	//frames []*frame
	id         uuid.UUID
	sourceName string
	bytes      []byte
	compressed bool

	// Reader state
	mu          sync.Mutex
	offset      uint
	frameNumber uint16
}

const MaxFrameCount = 64

var (
	ErrMixedCompression      = errors.New("mixed compression")
	ErrMaxFrameCountExceeded = errors.New("max frame count exceeded")
	ErrMtuTooSmall           = errors.New("mtu too small")
)

func newFrameQueue(sourceName string, bytes []byte) *frameQueue {
	return &frameQueue{
		id:          uuid.New(),
		sourceName:  sourceName,
		bytes:       bytes,
		compressed:  false,
		mu:          sync.Mutex{},
		offset:      0,
		frameNumber: 0,
	}
}

func (fq *frameQueue) ResetReader() {
	fq.mu.Lock()
	fq.frameNumber = 0
	fq.offset = 0
	fq.mu.Unlock()
}

func (fq *frameQueue) Next(mtu uint) (*frame, error) {
	fq.mu.Lock()
	defer fq.mu.Unlock()

	if fq.offset >= uint(len(fq.bytes)) {
		return nil, io.EOF
	}

	if fq.frameNumber >= MaxFrameCount {
		return nil, ErrMaxFrameCountExceeded
	}

	isFirstFrame := fq.frameNumber == 0
	availBytes, err := calculateMaxDataBytes(mtu, fq.sourceName)
	if err != nil {
		return nil, err
	}

	if availBytes < 1 {
		return nil, ErrMtuTooSmall
	}

	// If this is the first frame, check if we should compress
	if isFirstFrame && availBytes < uint(len(fq.bytes)) && !fq.compressed {
		compressed, err := compress(fq.bytes)
		if err != nil {
			return nil, err
		}

		fq.bytes = compressed
		fq.compressed = true
	}

	// Calculate byte range to send
	end := fq.offset + availBytes
	isLast := end >= uint(len(fq.bytes))
	if end > uint(len(fq.bytes)) {
		end = uint(len(fq.bytes))
	}

	// Create frame
	frame := newFrame(fq.compressed, fq.id, fq.frameNumber, isLast, fq.sourceName, fq.bytes[fq.offset:end])
	fq.frameNumber++
	fq.offset = end

	return frame, nil
}

func compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func decompress(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
