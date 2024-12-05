package swim

import (
	"bytes"
	"encoding/binary"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"hash/crc32"
	"io"
)

type frame struct {
	IsCompressed bool
	StreamId     uuid.UUID
	FrameNumber  uint16
	IsLast       bool
	Source       string
	Data         []byte
}

var ErrChecksumMismatch = errors.New("checksum mismatch")

func newFrame(isCompressed bool, streamId uuid.UUID, frameNumber uint16, isLast bool, source string, data []byte) *frame {
	return &frame{
		IsCompressed: isCompressed,
		StreamId:     streamId,
		FrameNumber:  frameNumber,
		IsLast:       isLast,
		Source:       source,
		Data:         data,
	}
}

func (f *frame) Marshal() ([]byte, error) {
	var buf bytes.Buffer

	if f.IsCompressed {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}

	buf.Write(f.StreamId[:])

	if err := binary.Write(&buf, binary.LittleEndian, f.FrameNumber); err != nil {
		return nil, err
	}

	if f.IsLast {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}

	sourceBytes := []byte(f.Source)
	if err := binary.Write(&buf, binary.LittleEndian, uint16(len(sourceBytes))); err != nil {
		return nil, err
	}
	buf.Write(sourceBytes)

	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(f.Data))); err != nil {
		return nil, err
	}
	buf.Write(f.Data)

	checksum := crc32.ChecksumIEEE(buf.Bytes())
	if err := binary.Write(&buf, binary.LittleEndian, checksum); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (f *frame) Unmarshal(data []byte) error {
	// Check data is long enough to contain the compressed flag, frame number, total frames, and source length
	if len(data) < 27 {
		return io.ErrUnexpectedEOF
	}

	offset := 0
	f.IsCompressed = data[0] == 1
	offset++

	f.StreamId = uuid.UUID(data[offset : offset+16])
	offset += 16

	f.FrameNumber = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	f.IsLast = data[offset] == 1
	offset++

	sourceLen := binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	if len(data) < offset+int(sourceLen) {
		return io.ErrUnexpectedEOF
	}

	f.Source = string(data[offset : offset+int(sourceLen)])
	offset += int(sourceLen)

	// Check data is long enough to contain the data length
	if len(data) < offset+2 {
		return io.ErrUnexpectedEOF
	}

	dataLen := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	if len(data) < offset+int(dataLen) {
		return io.ErrUnexpectedEOF
	}

	f.Data = data[offset : offset+int(dataLen)]
	offset += int(dataLen)

	// Check data is long enough to contain the checksum
	if len(data) < offset+4 {
		return io.ErrUnexpectedEOF
	}

	receivedChecksum := binary.LittleEndian.Uint32(data[offset : offset+4])

	// Calculate and compare checksum
	checksum := crc32.ChecksumIEEE(data[:offset])
	if checksum != receivedChecksum {
		return ErrChecksumMismatch
	}

	return nil
}

func calculateMaxDataBytes(mtu uint, sourceName string) (uint, error) {
	sourceBytes := len([]byte(sourceName))
	headerBytes := uint(1 + 16 + 2 + 1 + 2 + sourceBytes + 4 + 4)

	if mtu <= headerBytes {
		return 0, ErrMtuTooSmall
	}

	return mtu - headerBytes, nil
}
