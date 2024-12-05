package swim

import (
	"errors"
	"github.com/google/uuid"
	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
	"io"
	"math"
	"sync"
	"time"
)

type delegate struct {
	logger               *zap.Logger
	sourceName           string
	retransmitMultiplier int
	memberCount          func() int

	rx chan inboundMessage

	decoderMu sync.Mutex
	decoders  map[uuid.UUID]*frameQueueDecoder
	lastSeen  map[uuid.UUID]time.Time

	queueMu         sync.Mutex
	messageQueue    []*frameQueue
	retransmitQueue []*retransmissionFrame
	framesSeen      map[uuid.UUID]map[uint16]struct{}

	pruneShutdown chan struct{}
}

type inboundMessage struct {
	source string
	data   []byte
}

type retransmissionFrame struct {
	frame         *frame
	transmitCount int
}

const (
	WaitTimeout   = time.Second * 10
	PruneInterval = time.Second * 10
)

// Enforce interface constraints at compile time
var _ memberlist.Delegate = (*delegate)(nil)

func newDelegate(
	logger *zap.Logger,
	sourceName string,
	rx chan inboundMessage,
	retransmitMultiplier int,
	memberCount func() int,
) *delegate {
	return &delegate{
		logger:               logger,
		sourceName:           sourceName,
		retransmitMultiplier: retransmitMultiplier,
		memberCount:          memberCount,
		rx:                   rx,
		decoders:             make(map[uuid.UUID]*frameQueueDecoder),
		lastSeen:             make(map[uuid.UUID]time.Time),
		framesSeen:           make(map[uuid.UUID]map[uint16]struct{}),
		pruneShutdown:        make(chan struct{}),
	}
}

// Delegate interface methods

func (d *delegate) NodeMeta(limit int) []byte {
	return []byte{}
}

func (d *delegate) NotifyMsg(b []byte) {
	// Run handler async as to not block the UDP packet loop
	go d.handleInboundMessage(b)
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	d.queueMu.Lock()
	defer d.queueMu.Unlock()

	if len(d.messageQueue) == 0 && len(d.retransmitQueue) == 0 {
		return nil
	}

	mtu := limit - overhead
	if mtu < 1 {
		d.logger.Error("MTU too small", zap.Int("mtu", mtu), zap.Int("limit", limit), zap.Int("overhead", overhead))
		return nil
	}

	// Pop the first message off the queue
	var frame *frame
	for frame == nil {
		if len(d.messageQueue) == 0 {
			break // break not return to try a retransmission
		}

		fq := d.messageQueue[0]
		f, err := fq.Next(uint(mtu))
		if err != nil {
			if errors.Is(err, io.EOF) {
				d.messageQueue = d.messageQueue[1:]
				continue
			} else {
				d.logger.Error("Failed to get next frame from framequeue", zap.Error(err))
				d.messageQueue = d.messageQueue[1:] // Remove the message from the queue, as we won't be able to send the full message
				return nil
			}
		}

		frame = f
	}

	if frame == nil {
		// If there are no new frames, try retransmissions
		if len(d.retransmitQueue) == 0 {
			return nil
		}

		// Pop the first frame off the retransmission queue
		frame = d.retransmitQueue[0].frame
		d.retransmitQueue[0].transmitCount++

		d.logger.Debug(
			"Retransmitting frame",
			zap.Any("stream", frame.StreamId),
			zap.Uint16("frame_number", frame.FrameNumber),
			zap.ByteString("data", frame.Data),
		)

		removed := false
		if d.retransmitQueue[0].transmitCount >= d.getTransmissionCount() {
			d.retransmitQueue = d.retransmitQueue[1:]
			removed = true
		}

		bytes, err := frame.Marshal()
		if err != nil {
			d.logger.Error("Failed to marshal retransmission frame", zap.Error(err), zap.Any("frame", frame))
			if !removed {
				// Remove the frame from the queue, as this will be a persistent error
				d.retransmitQueue = d.retransmitQueue[1:]
			}

			return nil
		}

		return [][]byte{bytes}
	}

	if d.getTransmissionCount() > 1 {
		d.retransmitQueue = append(d.retransmitQueue, &retransmissionFrame{
			frame:         frame,
			transmitCount: 1,
		})
	}

	bytes, err := frame.Marshal()
	if err != nil {
		d.logger.Error("Failed to marshal frame", zap.Error(err), zap.Any("frame", frame))
		d.messageQueue = d.messageQueue[1:] // Remove the message from the queue, as we won't be able to send the full message
		return nil
	}

	return [][]byte{bytes}
}

func (d *delegate) LocalState(join bool) []byte {
	return nil
}

func (d *delegate) MergeRemoteState(buf []byte, join bool) {}

// Custom methods

func (d *delegate) Send(msg []byte) {
	fq := newFrameQueue(d.sourceName, msg)

	d.queueMu.Lock()
	d.messageQueue = append(d.messageQueue, fq)
	d.queueMu.Unlock()
}

func (d *delegate) handleInboundMessage(bytes []byte) {
	// Decode frame
	var f frame
	if err := f.Unmarshal(bytes); err != nil {
		d.logger.Error("Failed to unmarshal frame", zap.Error(err), zap.ByteString("frame", bytes))
		return
	}

	d.logger.Debug(
		"Received frame",
		zap.Bool("is_compressed", f.IsCompressed),
		zap.String("stream_id", f.StreamId.String()),
		zap.Uint16("frame_number", f.FrameNumber),
		zap.Bool("is_last", f.IsLast),
		zap.String("source", f.Source),
	)

	d.decoderMu.Lock()
	defer d.decoderMu.Unlock()

	decoder, ok := d.decoders[f.StreamId]
	if !ok {
		decoder = newFrameQueueDecoder(f.StreamId)
		d.decoders[f.StreamId] = decoder
		d.lastSeen[f.StreamId] = time.Now()
	}

	if f.Source != d.sourceName {
		d.queueMu.Lock()
		if frameNumsSeen, ok := d.framesSeen[f.StreamId]; ok {
			_, hasSeenFrame := frameNumsSeen[f.FrameNumber]
			if !hasSeenFrame {
				frameNumsSeen[f.FrameNumber] = struct{}{}
				d.retransmitQueue = append(d.retransmitQueue, &retransmissionFrame{
					frame:         &f,
					transmitCount: 0,
				})
			}
		} else {
			d.framesSeen[f.StreamId] = map[uint16]struct{}{
				f.FrameNumber: {},
			}
		}
		d.queueMu.Unlock()
	}

	// Don't process duplicate frames
	if decoder.HasReceivedFrame(f.FrameNumber) {
		return
	}

	if err := decoder.ReadBytes(bytes); err != nil {
		d.logger.Error("Failed to decode frame in framequeue context", zap.Error(err), zap.ByteString("frame", bytes))
		return
	}

	// No more writes after received all, so safe to not use a mutex
	if decoder.ReceivedAll() {
		d.logger.Debug("Received all frames for stream", zap.String("stream_id", f.StreamId.String()))
		delete(d.decoders, f.StreamId)
		delete(d.lastSeen, f.StreamId)

		decoded, err := decoder.Decode()
		if err != nil {
			d.logger.Error("Failed to decode frames", zap.Error(err))
			return
		}

		sourceName, err := decoder.SourceName()
		if err != nil {
			d.logger.Error("Failed to get source name from decoder", zap.Error(err))
			return
		}

		d.rx <- inboundMessage{
			source: sourceName,
			data:   decoded,
		}
	}
}

func (d *delegate) StopPruneLoop() {
	close(d.pruneShutdown)
}

func (d *delegate) StartPruneLoop() {
	timer := time.NewTimer(PruneInterval)

	for {
		select {
		case <-d.pruneShutdown:
			return
		case <-timer.C:
			d.decoderMu.Lock()
			d.queueMu.Lock()
			for id, lastSeen := range d.lastSeen {
				if time.Since(lastSeen) > WaitTimeout {
					d.logger.Warn(
						"Haven't seen stream for a long time, removing decoder state",
						zap.String("stream_id", id.String()),
					)

					delete(d.decoders, id)
					delete(d.lastSeen, id)
					delete(d.framesSeen, id)
				}
			}
			d.queueMu.Unlock()
			d.decoderMu.Unlock()
		}
	}
}

// getTransmissionCount returns the number of times a message should be transmitted (initial+retransmissions),
// based on the number of members in the cluster and the configurable retransmission multiplier.
func (d *delegate) getTransmissionCount() int {
	nodeCount := d.memberCount()
	return d.retransmitMultiplier * int(math.Ceil(math.Log10(float64(nodeCount+1))))
}
