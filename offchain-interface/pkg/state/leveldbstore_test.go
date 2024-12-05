package state

import (
	"bytes"
	"context"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"os"
	"testing"
	"time"
)

type LevelDBStoreSuite struct {
	suite.Suite
	tmpDir string
	db     *leveldb.DB
	store  *LevelDBStore
}

func (suite *LevelDBStoreSuite) SetupSuite() {
	tmpDir, err := os.MkdirTemp("", "leveldbstore_test")
	suite.Require().NoError(err)

	suite.tmpDir = tmpDir

	suite.db, err = leveldb.OpenFile(tmpDir, &opt.Options{
		Compression:         opt.NoCompression,
		CompactionL0Trigger: 0,
		NoWriteMerge:        true,
	})
	suite.Require().NoError(err)

	suite.store = &LevelDBStore{
		db: suite.db,
	}
}

func (suite *LevelDBStoreSuite) TearDownTest() {
	// Clear the database after each test
	iter := suite.db.NewIterator(nil, nil)
	for iter.Next() {
		suite.Require().NoError(suite.db.Delete(iter.Key(), nil))
	}
}

func (suite *LevelDBStoreSuite) TearDownSuite() {
	suite.Assert().NoError(suite.store.Close(context.Background()))
	suite.Assert().NoError(os.RemoveAll(suite.tmpDir))
}

func (suite *LevelDBStoreSuite) TestNilLastSeenBlockHeight() {
	height, err := suite.store.LastSeenBlockHeight(context.Background())
	suite.Require().NoError(err)
	suite.Require().Nil(height)
}

func (suite *LevelDBStoreSuite) TestSetLastSeenBlockHeight() {
	suite.Require().NoError(suite.store.SetLastSeenBlockHeight(context.Background(), 12345))

	height, err := suite.store.LastSeenBlockHeight(context.Background())
	suite.Require().NoError(err)
	suite.Require().NotNil(height)
	suite.Require().Equal(int64(12345), *height)
}

func (suite *LevelDBStoreSuite) TestEmptyMissingBlocks() {
	blocks, err := suite.store.MissingBlocks(context.Background())
	suite.Require().NoError(err)
	suite.Require().Empty(blocks)
}

func (suite *LevelDBStoreSuite) TestAddMissingBlocks() {
	range1 := NewBlockRange(1, 10)
	range2 := NewBlockRange(10, 20)
	range3 := NewBlockRange(20, 30)

	id1, err := suite.store.AddMissingBlocks(context.Background(), range1)
	suite.Require().NoError(err)

	id2, err := suite.store.AddMissingBlocks(context.Background(), range2)
	suite.Require().NoError(err)

	id3, err := suite.store.AddMissingBlocks(context.Background(), range3)
	suite.Require().NoError(err)

	blocks, err := suite.store.MissingBlocks(context.Background())
	suite.Require().NoError(err)
	suite.Require().Len(blocks, 3)

	suite.Require().Equal(id1, blocks[0].Id)
	suite.Require().Equal(range1, blocks[0].BlockRange)

	suite.Require().Equal(id2, blocks[1].Id)
	suite.Require().Equal(range2, blocks[1].BlockRange)

	suite.Require().Equal(id3, blocks[2].Id)
	suite.Require().Equal(range3, blocks[2].BlockRange)
}

func (suite *LevelDBStoreSuite) TestAddMissingBlocksMany() {
	for i := 0; i < 1_000; i++ {
		br := NewBlockRange(int64(i), int64(i))
		_, err := suite.store.AddMissingBlocks(context.Background(), br)
		suite.Require().NoError(err)
	}

	blocks, err := suite.store.MissingBlocks(context.Background())
	suite.Require().NoError(err)

	suite.Require().Len(blocks, 1_000)
	for i, block := range blocks {
		// Use values to validate order
		suite.Require().Equal(NewBlockRange(int64(i), int64(i)), block.BlockRange)
	}
}

func (suite *LevelDBStoreSuite) TestUpdateMissingBlocks() {
	range1 := NewBlockRange(1, 10)
	range2 := NewBlockRange(10, 20)
	range3 := NewBlockRange(20, 30)

	id1, err := suite.store.AddMissingBlocks(context.Background(), range1)
	suite.Require().NoError(err)

	id2, err := suite.store.AddMissingBlocks(context.Background(), range2)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.store.UpdateMissingBlockRange(
		context.Background(),
		NewBlockRangeWithId(range3, id2),
	))

	blocks, err := suite.store.MissingBlocks(context.Background())
	suite.Require().NoError(err)
	suite.Require().Len(blocks, 2)

	suite.Require().Equal(id1, blocks[0].Id)
	suite.Require().Equal(range1, blocks[0].BlockRange)

	suite.Require().Equal(id2, blocks[1].Id)
	suite.Require().Equal(range3, blocks[1].BlockRange)
}

func (suite *LevelDBStoreSuite) TestUpdateMissingBlocksNonExistent() {
	br := NewBlockRange(10, 20)
	suite.Require().NoError(suite.store.UpdateMissingBlockRange(
		context.Background(),
		NewBlockRangeWithId(br, [16]byte{0xff, 0xff}),
	))

	blocks, err := suite.store.MissingBlocks(context.Background())
	suite.Require().NoError(err)
	suite.Require().Len(blocks, 1)

	suite.Require().Equal([16]byte{0xff, 0xff}, blocks[0].Id)
	suite.Require().Equal(br, blocks[0].BlockRange)
}

func (suite *LevelDBStoreSuite) TestRemoveMissingBlocks() {
	range1 := NewBlockRange(1, 10)
	range2 := NewBlockRange(10, 20)
	range3 := NewBlockRange(20, 30)

	id1, err := suite.store.AddMissingBlocks(context.Background(), range1)
	suite.Require().NoError(err)

	id2, err := suite.store.AddMissingBlocks(context.Background(), range2)
	suite.Require().NoError(err)

	id3, err := suite.store.AddMissingBlocks(context.Background(), range3)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.store.RemoveMissingBlockRange(context.Background(), id2))

	blocks, err := suite.store.MissingBlocks(context.Background())
	suite.Require().NoError(err)
	suite.Require().Len(blocks, 2)

	suite.Require().Equal(id1, blocks[0].Id)
	suite.Require().Equal(range1, blocks[0].BlockRange)

	suite.Require().Equal(id3, blocks[1].Id)
	suite.Require().Equal(range3, blocks[1].BlockRange)
}

func (suite *LevelDBStoreSuite) TestRemoveMissingBlocksNonExistent() {
	suite.Require().NoError(suite.store.RemoveMissingBlockRange(context.Background(), [16]byte{0xfe}))
}

func (suite *LevelDBStoreSuite) TestEmptyMissingEvents() {
	retrieved, err := suite.store.MissingEvents(context.Background(), nil, 1_000)
	suite.Require().NoError(err)
	suite.Require().Empty(retrieved)
}

func (suite *LevelDBStoreSuite) TestAddMissingEvents() {
	ev1 := NewMissingEvent([]byte{0x01}, time.Now(), 1)
	ev2 := NewMissingEvent([]byte{0x02}, time.Now(), 1)
	ev3 := NewMissingEvent([]byte{0x03}, time.Now(), 1)

	inserted, err := suite.store.AddMissingEvents(context.Background(), ev1, ev2, ev3)
	suite.Require().NoError(err)

	suite.Require().Len(inserted, 3)
	suite.Require().Equal(ev1, inserted[0].MissingEvent)
	suite.Require().Equal(ev2, inserted[1].MissingEvent)
	suite.Require().Equal(ev3, inserted[2].MissingEvent)

	retrieved, err := suite.store.MissingEvents(context.Background(), nil, 1_000)
	suite.Require().NoError(err)
	requireEventsEqual(suite.T(), inserted, retrieved)
}

func (suite *LevelDBStoreSuite) TestAddMissingEventsDuplicates() {
	ev1 := NewMissingEvent([]byte{0x01}, time.Now(), 1)
	_, err := suite.store.AddMissingEvents(context.Background(), ev1)
	suite.Require().NoError(err)

	ev2 := NewMissingEvent([]byte{0x01}, time.Now(), 2) // ev2 should overwrite ev1
	ev3 := NewMissingEvent([]byte{0x03}, time.Now(), 1)

	inserted, err := suite.store.AddMissingEvents(context.Background(), ev2, ev3)
	suite.Require().NoError(err)

	suite.Require().Len(inserted, 2)
	suite.Require().Equal(ev2, inserted[0].MissingEvent)
	suite.Require().Equal(ev3, inserted[1].MissingEvent)

	retrieved, err := suite.store.MissingEvents(context.Background(), nil, 1_000)
	suite.Require().NoError(err)
	requireEventsEqual(suite.T(), inserted, retrieved)
}

func (suite *LevelDBStoreSuite) TestIncreaseRetryCountOne() {
	ev1 := NewMissingEvent([]byte{0x01}, time.Now(), 1)
	_, err := suite.store.AddMissingEvents(context.Background(), ev1)
	suite.Require().NoError(err)

	suite.Require().Equal(false, ev1.RetriedUnicast)
	suite.Require().Equal(0, ev1.RetryCount)

	suite.Require().NoError(suite.store.IncrementMissingEventRetryCount(context.Background(), ev1.EventId))

	retrieved, err := suite.store.MissingEvents(context.Background(), nil, 1_000)
	suite.Require().NoError(err)

	suite.Require().Len(retrieved, 1)

	retrievedEvent := retrieved[0].MissingEvent
	suite.Require().NotEqual(ev1.LastRetryTime, retrievedEvent.LastRetryTime)
	suite.Require().Equal(true, retrievedEvent.RetriedUnicast)
	suite.Require().Equal(1, retrievedEvent.RetryCount)
}

func (suite *LevelDBStoreSuite) TestIncreaseRetryCountTen() {
	ev1 := NewMissingEvent([]byte{0x01}, time.Now(), 1)
	_, err := suite.store.AddMissingEvents(context.Background(), ev1)
	suite.Require().NoError(err)

	suite.Require().Equal(false, ev1.RetriedUnicast)
	suite.Require().Equal(0, ev1.RetryCount)

	for i := 0; i < 10; i++ {
		suite.Require().NoError(suite.store.IncrementMissingEventRetryCount(context.Background(), ev1.EventId))
	}

	retrieved, err := suite.store.MissingEvents(context.Background(), nil, 1_000)
	suite.Require().NoError(err)

	suite.Require().Len(retrieved, 1)

	retrievedEvent := retrieved[0].MissingEvent
	suite.Require().NotEqual(ev1.LastRetryTime, retrievedEvent.LastRetryTime)
	suite.Require().Equal(true, retrievedEvent.RetriedUnicast)
	suite.Require().Equal(10, retrievedEvent.RetryCount)
}

func (suite *LevelDBStoreSuite) TestRemoveMissingEvents() {
	ev1 := NewMissingEvent([]byte{0x01}, time.Now(), 1)
	ev2 := NewMissingEvent([]byte{0x02}, time.Now(), 1)
	ev3 := NewMissingEvent([]byte{0x03}, time.Now(), 1)

	inserted, err := suite.store.AddMissingEvents(context.Background(), ev1, ev2, ev3)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.store.RemoveMissingEvent(context.Background(), ev1.EventId))
	suite.Require().NoError(suite.store.RemoveMissingEvent(context.Background(), ev2.EventId))

	retrieved, err := suite.store.MissingEvents(context.Background(), nil, 1_000)
	suite.Require().NoError(err)

	requireEventsEqual(suite.T(), inserted[2:], retrieved)
}

func (suite *LevelDBStoreSuite) TestRemoveMissingEventsNonExistent() {
	suite.Require().NoError(suite.store.RemoveMissingEvent(context.Background(), events.EventHash{1}))
}

func (suite *LevelDBStoreSuite) TestGetMissingEventsLimit() {
	missingEvents := make([]MissingEvent, 10)
	for i := 0; i < 10; i++ {
		missingEvents[i] = NewMissingEvent([]byte{byte(i)}, time.Now(), 1)
	}

	inserted, err := suite.store.AddMissingEvents(context.Background(), missingEvents...)
	suite.Require().NoError(err)

	for i, ev := range inserted {
		suite.Require().Equal(missingEvents[i].EventId, ev.MissingEvent.EventId)
	}

	retrieved, err := suite.store.MissingEvents(context.Background(), nil, 5)
	suite.Require().NoError(err)

	require.Len(suite.T(), retrieved, 5)
	requireEventsEqual(suite.T(), inserted[:5], retrieved)
}

func (suite *LevelDBStoreSuite) TestGetMissingEventsSearch() {
	missingEvents := make([]MissingEvent, 10)
	for i := 0; i < 10; i++ {
		missingEvents[i] = NewMissingEvent([]byte{byte(i)}, time.Now(), 1)
	}

	inserted, err := suite.store.AddMissingEvents(context.Background(), missingEvents...)
	suite.Require().NoError(err)

	for i, ev := range inserted {
		suite.Require().Equal(missingEvents[i].EventId, ev.MissingEvent.EventId)
	}

	retrieved, err := suite.store.MissingEvents(context.Background(), &inserted[4].Id, 5)
	suite.Require().NoError(err)

	require.Len(suite.T(), retrieved, 5)
	requireEventsEqual(suite.T(), inserted[5:], retrieved)
}

func (suite *LevelDBStoreSuite) TestGetMissingEventsSearchNone() {
	missingEvents := make([]MissingEvent, 10)
	for i := 0; i < 10; i++ {
		missingEvents[i] = NewMissingEvent([]byte{byte(i)}, time.Now(), 1)
	}

	inserted, err := suite.store.AddMissingEvents(context.Background(), missingEvents...)
	suite.Require().NoError(err)
	suite.Require().Len(inserted, 10)

	retrieved, err := suite.store.MissingEvents(context.Background(), &[16]byte{0xff, 0xff}, 100)
	suite.Require().NoError(err)

	require.Len(suite.T(), retrieved, 0)
}

func (suite *LevelDBStoreSuite) TestGetMissingEventsNegativeLimit() {
	_, err := suite.store.MissingEvents(context.Background(), nil, -1)
	suite.Require().Error(err)
}

func TestLevelDBStoreSuite(t *testing.T) {
	suite.Run(t, new(LevelDBStoreSuite))
}

func TestGetNextKey(t *testing.T) {
	currentKey := [16]byte{}

	for i := 0; i < 1_000_000; i++ {
		nextKey := getNextKey(currentKey)
		assert.Truef(t, bytes.Compare(nextKey[:], currentKey[:]) > 0, "i: %d, nextKey: %v, currentKey: %v", i, nextKey, currentKey)
		currentKey = nextKey
	}
}

func requireEventsEqual[T any](t *testing.T, expected []MissingEventWithKey[T], actual []MissingEventWithKey[T]) {
	t.Helper()
	require.Len(t, actual, len(expected))

	for i, ev := range expected {
		require.Equal(t, ev.MissingEvent.EventId, actual[i].MissingEvent.EventId)
		// Times cannot be compared directly as structs, due to monotonic clock
		require.True(t, ev.MissingEvent.ReceivedTime.Equal(actual[i].MissingEvent.ReceivedTime))
		require.Equal(t, ev.MissingEvent.BlockHeight, actual[i].MissingEvent.BlockHeight)
	}
}
