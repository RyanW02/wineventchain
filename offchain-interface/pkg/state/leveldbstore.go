package state

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/offchain-interface/internal/config"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
	"time"
)

type LevelDBStore struct {
	db *leveldb.DB
}

const (
	keyLastSeenBlockHeight      = "last_seen_block_height"
	keyMissingBlocksIdCounter   = "id_counter_missing_blocks"
	keyPrefixMissingBlocks      = "missing_blocks_"
	keyMissingEventsIdCounter   = "id_counter_missing_events"
	keyPrefixMissingEvents      = "missing_events_"
	keyPrefixMissingEventsIndex = "index_missing_events_"
)

// Enforce interface constraints at compile time
var _ Store[[16]byte, [16]byte] = (*LevelDBStore)(nil)

func NewLevelDBStore(cfg config.Config) (*LevelDBStore, error) {
	db, err := leveldb.OpenFile(cfg.State.Path, nil)
	if err != nil {
		return nil, err
	}

	return &LevelDBStore{
		db: db,
	}, nil
}

func (s *LevelDBStore) Close(ctx context.Context) error {
	return s.db.Close()
}

func (s *LevelDBStore) LastSeenBlockHeight(ctx context.Context) (*int64, error) {
	value, err := s.db.Get(bz(keyLastSeenBlockHeight), nil)
	if err != nil {
		if errors.Is(err, leveldb.ErrNotFound) {
			return nil, nil
		}

		return nil, err
	}

	height := bytesToInt(value)
	return &height, nil
}

func (s *LevelDBStore) SetLastSeenBlockHeight(ctx context.Context, height int64) error {
	heightBytes := int64ToBytes(height)
	return s.db.Put(bz(keyLastSeenBlockHeight), heightBytes, nil)
}

func (s *LevelDBStore) MissingBlocks(ctx context.Context) ([]BlockRangeWithId[[16]byte], error) {
	it := s.db.NewIterator(util.BytesPrefix(bz(keyPrefixMissingBlocks)), &opt.ReadOptions{
		DontFillCache: false,
		Strict:        0,
	})
	defer it.Release()

	var missingBlocks []BlockRangeWithId[[16]byte]
	for it.Next() {
		key := make([]byte, len(it.Key()))
		copy(key, it.Key())

		key = key[len(keyPrefixMissingBlocks):]
		if len(key) != 16 {
			return nil, fmt.Errorf("invalid key length for missing block range")
		}

		id := [16]byte(key)

		if len(it.Value()) != 16 {
			return nil, fmt.Errorf("invalid value length for missing block range")
		}

		low := bytesToInt(it.Value()[:8])
		high := bytesToInt(it.Value()[8:])
		blockRange := BlockRange{
			Low:  low,
			High: high,
		}

		missingBlocks = append(missingBlocks, NewBlockRangeWithId(blockRange, id))
	}

	return missingBlocks, nil
}

func (s *LevelDBStore) AddMissingBlocks(ctx context.Context, blockRange BlockRange) ([16]byte, error) {
	return s.withIncrementingId(keyMissingBlocksIdCounter, func(tx *leveldb.Transaction, id [16]byte) error {
		key := append(bz(keyPrefixMissingBlocks), id[:]...)

		low := int64ToBytes(blockRange.Low)
		high := int64ToBytes(blockRange.High)
		encoded := append(low, high...)

		return tx.Put(key, encoded, nil)
	})
}

func (s *LevelDBStore) UpdateMissingBlockRange(ctx context.Context, blockRange BlockRangeWithId[[16]byte]) error {
	key := append(bz(keyPrefixMissingBlocks), blockRange.Id[:]...)

	lowEncoded := int64ToBytes(blockRange.Low)
	highEncoded := int64ToBytes(blockRange.High)
	encoded := append(lowEncoded, highEncoded...)

	return s.db.Put(key, encoded, nil)
}

func (s *LevelDBStore) RemoveMissingBlockRange(ctx context.Context, id [16]byte) error {
	key := append(bz(keyPrefixMissingBlocks), id[:]...)
	return s.db.Delete(key, nil)
}

func (s *LevelDBStore) MissingEvents(ctx context.Context, after *[16]byte, limit int) ([]MissingEventWithKey[[16]byte], error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than 0")
	}

	it := s.db.NewIterator(util.BytesPrefix(bz(keyPrefixMissingEvents)), nil)
	defer it.Release()

	if after != nil {
		afterKey := append(bz(keyPrefixMissingEvents), after[:]...)
		it.Seek(afterKey)
	}

	var missingEvents []MissingEventWithKey[[16]byte]
	for it.Next() && len(missingEvents) < limit {
		idRaw := bytes.TrimPrefix(it.Key(), bz(keyPrefixMissingEvents))
		if len(idRaw) != 16 {
			return nil, fmt.Errorf("invalid key length for missing event")
		}

		id := [16]byte(idRaw)

		// Iterator#Seek jumps to the first result greater than or *equal to* the given key. We want to start
		// iterating from the next result after the `after` key given in the parameters, so we skip the first
		// result if it matches the `after` key.
		if after != nil && bytes.Equal((*after)[:], id[:]) {
			continue
		}

		value := make([]byte, len(it.Value()))
		copy(value, it.Value())

		missingEvent, err := unmarshalMissingEvent(value)
		if err != nil {
			return nil, err
		}

		missingEvents = append(missingEvents, MissingEventWithKey[[16]byte]{
			MissingEvent: missingEvent,
			Id:           id,
		})
	}

	return missingEvents, nil
}

func (s *LevelDBStore) MissingEventCount(ctx context.Context) (int, error) {
	it := s.db.NewIterator(util.BytesPrefix(bz(keyPrefixMissingEventsIndex)), nil)
	defer it.Release()

	count := 0
	for it.Next() {
		count++
	}

	return count, nil
}

func (s *LevelDBStore) AddMissingEvents(ctx context.Context, events ...MissingEvent) ([]MissingEventWithKey[[16]byte], error) {
	var i int
	eventsWithId := make([]MissingEventWithKey[[16]byte], len(events))
	if err := s.withIncrementingIdBatch(keyMissingEventsIdCounter, len(events), func(tx *leveldb.Transaction, batch *leveldb.Batch, internalId [16]byte) error {
		event := events[i]
		eventsWithId[i] = MissingEventWithKey[[16]byte]{
			MissingEvent: event,
			Id:           internalId,
		}
		i++

		eventId := []byte(event.EventId)

		// Add index for event ID
		indexKey := append(bz(keyPrefixMissingEventsIndex), eventId...)
		prevInternalId, err := tx.Get(indexKey, nil)
		exists := true
		if err != nil {
			if errors.Is(err, leveldb.ErrNotFound) {
				exists = false
			} else {
				return err
			}
		}

		// If exists, overwrite
		batch.Put(indexKey, internalId[:])

		if exists {
			// Delete previous event
			key := append(bz(keyPrefixMissingEvents), prevInternalId...)
			batch.Delete(key)
		}

		encoded, err := marshalMissingEvent(event)
		if err != nil {
			return err
		}

		key := append(bz(keyPrefixMissingEvents), internalId[:]...)
		batch.Put(key, encoded)

		return nil
	}); err != nil {
		return nil, err
	}

	return eventsWithId, nil
}

func (s *LevelDBStore) IncrementMissingEventRetryCount(ctx context.Context, eventId events.EventHash) error {
	return s.withTransaction(func(tx *leveldb.Transaction) error {
		// Get key for event ID
		indexKey := append(bz(keyPrefixMissingEventsIndex), []byte(eventId)...)
		internalId, err := tx.Get(indexKey, nil)
		if err != nil {
			if errors.Is(err, leveldb.ErrNotFound) {
				return fmt.Errorf("error incrementing missing event retry count: event index not found")
			}

			return err
		}

		key := append(bz(keyPrefixMissingEvents), internalId...)
		value, err := tx.Get(key, nil)
		if err != nil {
			return err
		}

		missingEvent, err := unmarshalMissingEvent(value)
		if err != nil {
			return err
		}

		// Only retry unicast once
		if !missingEvent.RetriedUnicast {
			missingEvent.RetriedUnicast = true
		}

		missingEvent.LastRetryTime = time.Now()
		missingEvent.RetryCount++

		encoded, err := marshalMissingEvent(missingEvent)
		if err != nil {
			return err
		}

		return tx.Put(key, encoded, nil)
	})
}

func (s *LevelDBStore) RemoveMissingEvent(ctx context.Context, eventId events.EventHash) error {
	return s.withTransaction(func(tx *leveldb.Transaction) error {
		// Get key for event ID
		indexKey := append(bz(keyPrefixMissingEventsIndex), []byte(eventId)...)
		internalId, err := tx.Get(indexKey, nil)
		if err != nil {
			if errors.Is(err, leveldb.ErrNotFound) {
				return nil
			}

			return err
		}

		if err := tx.Delete(indexKey, nil); err != nil {
			return err
		}

		key := append(bz(keyPrefixMissingEvents), internalId...)
		if err := tx.Delete(key, nil); err != nil && !errors.Is(err, leveldb.ErrNotFound) {
			return err
		}

		return nil
	})
}

func (s *LevelDBStore) withIncrementingId(counterKey string, f func(tx *leveldb.Transaction, id [16]byte) error) ([16]byte, error) {
	var id [16]byte
	if err := s.withTransaction(func(tx *leveldb.Transaction) error {
		// Get next ID
		counterBytes, err := tx.Get(bz(counterKey), nil)
		if err == nil {
			if len(counterBytes) != 16 {
				return fmt.Errorf("invalid counter length: %d", len(counterBytes))
			}

			id = [16]byte(counterBytes)
		} else {
			if errors.Is(err, leveldb.ErrNotFound) {
				id = [16]byte{}
			} else {
				return err
			}
		}

		id = getNextKey(id)

		if err := tx.Put(bz(counterKey), id[:], nil); err != nil {
			return err
		}

		return f(tx, id)
	}); err != nil {
		return [16]byte{}, err
	}

	return id, nil
}

func (s *LevelDBStore) withIncrementingIdBatch(counterKey string, n int, f func(tx *leveldb.Transaction, batch *leveldb.Batch, id [16]byte) error) error {
	return s.withTransaction(func(tx *leveldb.Transaction) error {
		// Get next ID
		var id [16]byte
		counterBytes, err := tx.Get(bz(counterKey), nil)
		if err == nil {
			if len(counterBytes) != 16 {
				return fmt.Errorf("invalid counter length: %d", len(counterBytes))
			}

			id = [16]byte(counterBytes)
		} else {
			if errors.Is(err, leveldb.ErrNotFound) {
				id = [16]byte{}
			} else {
				return err
			}
		}

		ids := make([][16]byte, n)
		for i := 0; i < n; i++ {
			id = getNextKey(id)
			ids[i] = id
		}

		if err := tx.Put(bz(counterKey), id[:], nil); err != nil {
			return err
		}

		batch := new(leveldb.Batch)
		for _, id := range ids {
			if err := f(tx, batch, id); err != nil {
				return err
			}
		}

		return tx.Write(batch, nil)
	})
}

func (s *LevelDBStore) withTransaction(f func(tx *leveldb.Transaction) error) error {
	tx, err := s.db.OpenTransaction()
	if err != nil {
		return err
	}

	defer tx.Discard()

	if err := f(tx); err != nil {
		return err
	}

	return tx.Commit()
}

func marshalMissingEvent(event MissingEvent) ([]byte, error) {
	receivedTime, err := event.ReceivedTime.MarshalBinary()
	if err != nil {
		return nil, err
	}

	lastRetryTime, err := event.ReceivedTime.MarshalBinary()
	if err != nil {
		return nil, err
	}

	idLen := int16ToBytes(int16(len(event.EventId)))
	receivedTimeLen := int16ToBytes(int16(len(receivedTime)))
	blockHeight := int64ToBytes(event.BlockHeight)
	lastRetryTimeLen := int16ToBytes(int16(len(lastRetryTime)))
	retriedUnicast := boolToBytes(event.RetriedUnicast)
	retryCount := int64ToBytes(int64(event.RetryCount))

	return join(
		idLen,
		event.EventId,
		receivedTimeLen,
		receivedTime,
		blockHeight,
		lastRetryTimeLen,
		lastRetryTime,
		retriedUnicast,
		retryCount,
	), nil
}

func unmarshalMissingEvent(bytes []byte) (MissingEvent, error) {
	if len(bytes) < 2 {
		return MissingEvent{}, fmt.Errorf("invalid bytes length for missing event")
	}

	offset := 0
	idLen := bytesToInt16(bytes[:2])
	offset += 2

	if len(bytes) < offset+int(idLen)+2 {
		return MissingEvent{}, fmt.Errorf("invalid bytes length for missing event")
	}

	eventId := bytes[offset : offset+int(idLen)]
	offset += int(idLen)

	receivedTimeLen := bytesToInt16(bytes[offset : offset+2])
	offset += 2

	if len(bytes) < offset+int(receivedTimeLen)+8+2 {
		return MissingEvent{}, fmt.Errorf("invalid bytes length for missing event")
	}

	receivedTimeBytes := bytes[offset : offset+int(receivedTimeLen)]
	offset += int(receivedTimeLen)

	blockHeight := bytesToInt(bytes[offset : offset+8])
	offset += 8

	lastRetryTimeLen := bytesToInt16(bytes[offset : offset+2])
	offset += 2

	if len(bytes) < offset+int(lastRetryTimeLen)+1+8 {
		return MissingEvent{}, fmt.Errorf("invalid bytes length for missing event")
	}

	lastRetryTimeBytes := bytes[offset : offset+int(lastRetryTimeLen)]
	offset += int(lastRetryTimeLen)

	retriedUnicast := bytes[offset] == 1
	offset++

	retryCount := bytesToInt(bytes[offset : offset+8])

	var receivedTime time.Time
	if err := receivedTime.UnmarshalBinary(receivedTimeBytes); err != nil {
		return MissingEvent{}, err
	}

	var lastRetryTime time.Time
	if err := lastRetryTime.UnmarshalBinary(lastRetryTimeBytes); err != nil {
		return MissingEvent{}, err
	}

	return MissingEvent{
		EventId:        eventId,
		ReceivedTime:   receivedTime,
		BlockHeight:    blockHeight,
		LastRetryTime:  lastRetryTime,
		RetriedUnicast: retriedUnicast,
		RetryCount:     int(retryCount),
	}, nil
}

func join(bytes ...[]byte) []byte {
	var result []byte
	for _, b := range bytes {
		result = append(result, b...)
	}
	return result
}

func boolToBytes(b bool) []byte {
	if b {
		return []byte{1}
	} else {
		return []byte{0}
	}
}

func int16ToBytes(i int16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, uint16(i))
	return b
}

func int64ToBytes(i int64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(i))
	return b
}

func uint64ToBytes(i uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, i)
	return b
}

func bytesToInt16(b []byte) int16 {
	return int16(binary.LittleEndian.Uint16(b))
}

func bytesToInt(b []byte) int64 {
	return int64(binary.LittleEndian.Uint64(b))
}

func bz(s string) []byte {
	return []byte(s)
}

func getNextKey(currentKey [16]byte) [16]byte {
	var nextKey [16]byte
	copy(nextKey[:], currentKey[:])

	// Increment the last byte that is not 255 by 1.
	for i := len(nextKey) - 1; i >= 0; i-- {
		if nextKey[i] == 255 {
			// If we move forward a place, set all places after it to 0. e.g. [0, 255, 255] -> [1, 0, 0].
			for j := i; j < len(nextKey); j++ {
				nextKey[j] = 0
			}

			continue
		}

		nextKey[i]++
		return nextKey
	}

	// Loop back around to 0, should never happen: there are 8^16 values.
	return [16]byte{0}
}
