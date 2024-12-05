package mongodb

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"github.com/RyanW02/wineventchain/common/pkg/test"
	"github.com/RyanW02/wineventchain/common/pkg/types"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
	"github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	"github.com/RyanW02/wineventchain/common/pkg/utils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"testing"
	"time"
)

type RetentionTestSuite struct {
	test.MongoSuite
}

const (
	ChannelSecurity = "security"
	ChannelSystem   = "system"
)

func TestRetentionSuite(t *testing.T) {
	suite.Run(t, new(RetentionTestSuite))
}

func (suite *RetentionTestSuite) SetupTest() {
	suite.MongoSuite.SetupTest()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger, err := zap.NewDevelopment()
	suite.Require().NoErrorf(err, "Could not create logger")

	r := &MongoEventRepository{
		logger:     logger,
		collection: suite.Collection(),
	}
	suite.Require().NoErrorf(r.InitSchema(ctx), "Could not initialize schema")
}

func (suite *RetentionTestSuite) TestRetainTimestamp() {
	policy := offchain.RetentionPolicy{
		Filters: []offchain.Filter{
			{
				PolicyAction: offchain.PolicyAction{
					Type:            offchain.PolicyTypeTimestamp,
					RuleGroup:       offchain.RuleGroupingGlobal,
					RetentionPeriod: types.MarshalledDuration(time.Hour),
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var testData []any
	var hashes []events.EventHash
	for i := 0; i < 100; i++ {
		eventTime := time.Now()
		if i < 30 {
			eventTime = eventTime.Add(-24 * time.Hour)
		}

		ev := generateStoredEvent(suite.T(), i, eventTime, "principal", ChannelSecurity, i, nil)
		testData = append(testData, ev)
		hashes = append(hashes, ev.Metadata.EventId)
	}

	_, err := suite.Collection().InsertMany(ctx, testData)
	suite.Require().NoErrorf(err, "Could not insert test data: %s", err)

	aggregate, err := buildAggregate(policy)
	suite.Require().NoErrorf(err, "Could not build aggregate: %s", err)

	cursor, err := suite.Collection().Aggregate(ctx, aggregate)
	suite.Require().NoErrorf(err, "Could not aggregate: %s", err)

	requireHashes(suite.T(), cursor, hashes[:30])
}

func (suite *RetentionTestSuite) TestRetainVolume() {
	policy := offchain.RetentionPolicy{
		Filters: []offchain.Filter{
			{
				PolicyAction: offchain.PolicyAction{
					Type:      offchain.PolicyTypeCount,
					RuleGroup: offchain.RuleGroupingGlobal,
					Volume:    50,
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var testData []any
	var hashes []events.EventHash
	for i := 0; i < 100; i++ {
		ev := generateStoredEvent(suite.T(), i, time.Now().Add(-time.Minute*time.Duration(i)), "principal", ChannelSecurity, i, nil)
		testData = append(testData, ev)
		hashes = append(hashes, ev.Metadata.EventId)
	}

	_, err := suite.Collection().InsertMany(ctx, testData)
	suite.Require().NoErrorf(err, "Could not insert test data: %s", err)

	aggregate, err := buildAggregate(policy)
	suite.Require().NoErrorf(err, "Could not build aggregate: %s", err)

	cursor, err := suite.Collection().Aggregate(ctx, aggregate)
	suite.Require().NoErrorf(err, "Could not aggregate: %s", err)

	// hashes goes newest -> oldest, so last 50 items should be removed
	expected := hashes[50:]
	requireHashes(suite.T(), cursor, expected)
}

func (suite *RetentionTestSuite) TestRetainVolumeAndTimestamp() {
	policy := offchain.RetentionPolicy{
		Filters: []offchain.Filter{
			{
				PolicyAction: offchain.PolicyAction{
					Type:      offchain.PolicyTypeCount,
					RuleGroup: offchain.RuleGroupingGlobal,
					Volume:    50,
				},
			},
			{
				PolicyAction: offchain.PolicyAction{
					Type:            offchain.PolicyTypeTimestamp,
					RuleGroup:       offchain.RuleGroupingGlobal,
					RetentionPeriod: types.MarshalledDuration(time.Hour * 24 * 30),
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var testData []any
	var hashes []events.EventHash
	for i := 0; i < 100; i++ {
		eventTime := time.Now()
		if i < 70 {
			eventTime = eventTime.Add(-time.Hour*24*60 - time.Minute*time.Duration(i))
		}

		ev := generateStoredEvent(suite.T(), i, eventTime, "principal", ChannelSecurity, i, nil)
		testData = append(testData, ev)
		hashes = append(hashes, ev.Metadata.EventId)
	}

	_, err := suite.Collection().InsertMany(ctx, testData)
	suite.Require().NoErrorf(err, "Could not insert test data: %s", err)

	aggregate, err := buildAggregate(policy)
	suite.Require().NoErrorf(err, "Could not build aggregate: %s", err)

	cursor, err := suite.Collection().Aggregate(ctx, aggregate)
	suite.Require().NoErrorf(err, "Could not aggregate: %s", err)

	// Last 30 are time.Now(), 0 -> 70 are newest -> oldest. Policy has 50 overflow capacity
	expected := hashes[50:70]
	requireHashes(suite.T(), cursor, expected)
}

func (suite *RetentionTestSuite) TestRetainVolumeGrouped() {
	policy := offchain.RetentionPolicy{
		Filters: []offchain.Filter{
			{
				PolicyAction: offchain.PolicyAction{
					Type:      offchain.PolicyTypeCount,
					RuleGroup: offchain.RuleGroupingPrincipal,
					Volume:    3, // 3 events per principal
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var testData []any
	var hashes []events.EventHash
	principals := []string{"p1", "p2", "p3"}
	for i := 0; i < 99; i++ {
		ev := generateStoredEvent(suite.T(), i, time.Now().Add(-time.Minute*time.Duration(i)), principals[i%3], ChannelSecurity, i, nil)
		testData = append(testData, ev)
		hashes = append(hashes, ev.Metadata.EventId)
	}

	_, err := suite.Collection().InsertMany(ctx, testData)
	suite.Require().NoErrorf(err, "Could not insert test data: %s", err)

	aggregate, err := buildAggregate(policy)
	suite.Require().NoErrorf(err, "Could not build aggregate: %s", err)

	cursor, err := suite.Collection().Aggregate(ctx, aggregate)
	suite.Require().NoErrorf(err, "Could not aggregate: %s", err)

	expected := hashes[9:]
	requireHashes(suite.T(), cursor, expected)
}

func (suite *RetentionTestSuite) TestRetainChannelTimestamp() {
	policy := offchain.RetentionPolicy{
		Filters: []offchain.Filter{
			{
				Match: offchain.Match{
					Channel: utils.Ptr("SyStEm"), // Test case insensitivity
				},
				PolicyAction: offchain.PolicyAction{
					Type:            offchain.PolicyTypeTimestamp,
					RuleGroup:       offchain.RuleGroupingGlobal,
					RetentionPeriod: types.MarshalledDuration(time.Hour * 24 * 90),
				},
			},
			{
				PolicyAction: offchain.PolicyAction{
					Type:            offchain.PolicyTypeTimestamp,
					RuleGroup:       offchain.RuleGroupingGlobal,
					RetentionPeriod: types.MarshalledDuration(time.Hour * 24 * 30),
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var testData []any
	var hashes []events.EventHash
	for i := 0; i < 100; i++ {
		eventTime := time.Now().Add(-time.Hour * 24 * 60)

		channel := ChannelSecurity
		if i < 50 {
			channel = ChannelSystem
		}

		ev := generateStoredEvent(suite.T(), i, eventTime, "principal", channel, i, nil)
		testData = append(testData, ev)
		hashes = append(hashes, ev.Metadata.EventId)
	}

	_, err := suite.Collection().InsertMany(ctx, testData)
	suite.Require().NoErrorf(err, "Could not insert test data: %s", err)

	aggregate, err := buildAggregate(policy)
	suite.Require().NoErrorf(err, "Could not build aggregate: %s", err)

	cursor, err := suite.Collection().Aggregate(ctx, aggregate, options.Aggregate().SetCollation(collationCaseInsensitive))
	suite.Require().NoErrorf(err, "Could not aggregate: %s", err)

	expected := hashes[50:] // Delete events with "Security" channel
	requireHashes(suite.T(), cursor, expected)
}

func (suite *RetentionTestSuite) TestRetainEventIdTimestamp() {
	policy := offchain.RetentionPolicy{
		Filters: []offchain.Filter{
			{
				Match: offchain.Match{
					EventId: utils.Ptr(events.EventId(300)),
				},
				PolicyAction: offchain.PolicyAction{
					Type:            offchain.PolicyTypeTimestamp,
					RuleGroup:       offchain.RuleGroupingGlobal,
					RetentionPeriod: types.MarshalledDuration(time.Hour * 24 * 90),
				},
			},
			{
				PolicyAction: offchain.PolicyAction{
					Type:            offchain.PolicyTypeTimestamp,
					RuleGroup:       offchain.RuleGroupingGlobal,
					RetentionPeriod: types.MarshalledDuration(time.Hour * 24 * 30),
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var testData []any
	var hashes []events.EventHash
	for i := 0; i < 100; i++ {
		eventTime := time.Now().Add(-time.Hour * 24 * 60)

		eventId := i
		if i < 30 {
			eventId = 300
		}

		ev := generateStoredEvent(suite.T(), i, eventTime, "p", ChannelSecurity, eventId, nil)
		testData = append(testData, ev)
		hashes = append(hashes, ev.Metadata.EventId)
	}

	_, err := suite.Collection().InsertMany(ctx, testData)
	suite.Require().NoErrorf(err, "Could not insert test data: %s", err)

	aggregate, err := buildAggregate(policy)
	suite.Require().NoErrorf(err, "Could not build aggregate: %s", err)

	cursor, err := suite.Collection().Aggregate(ctx, aggregate)
	suite.Require().NoErrorf(err, "Could not aggregate: %s", err)

	expected := hashes[30:] // Delete events without event ID 300
	requireHashes(suite.T(), cursor, expected)
}

func (suite *RetentionTestSuite) TestRetainProviderTimestamp() {
	provider := events.NewGuid(uuid.New())

	policy := offchain.RetentionPolicy{
		Filters: []offchain.Filter{
			{
				Match: offchain.Match{
					ProviderGuid: utils.Ptr(provider.String()),
				},
				PolicyAction: offchain.PolicyAction{
					Type:            offchain.PolicyTypeTimestamp,
					RuleGroup:       offchain.RuleGroupingGlobal,
					RetentionPeriod: types.MarshalledDuration(time.Hour * 24 * 90),
				},
			},
			{
				PolicyAction: offchain.PolicyAction{
					Type:            offchain.PolicyTypeTimestamp,
					RuleGroup:       offchain.RuleGroupingGlobal,
					RetentionPeriod: types.MarshalledDuration(time.Hour * 24 * 30),
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var testData []any
	var hashes []events.EventHash
	for i := 0; i < 100; i++ {
		eventTime := time.Now().Add(-time.Hour * 24 * 60)

		var eventProvider *events.Guid
		if i >= 70 {
			eventProvider = provider
		}

		ev := generateStoredEvent(suite.T(), i, eventTime, "p", ChannelSecurity, i, eventProvider)
		testData = append(testData, ev)
		hashes = append(hashes, ev.Metadata.EventId)
	}

	_, err := suite.Collection().InsertMany(ctx, testData)
	suite.Require().NoErrorf(err, "Could not insert test data: %s", err)

	aggregate, err := buildAggregate(policy)
	suite.Require().NoErrorf(err, "Could not build aggregate: %s", err)

	cursor, err := suite.Collection().Aggregate(ctx, aggregate)
	suite.Require().NoErrorf(err, "Could not aggregate: %s", err)

	expected := hashes[:70]
	requireHashes(suite.T(), cursor, expected)
}

func (suite *RetentionTestSuite) TestRetainComplexLogicTimestamp() {
	provider := events.NewGuid(uuid.New())

	policy := offchain.RetentionPolicy{
		// Retain for 90d: channel="security" AND provider="..."
		// Retail for 60d: event_id=12345
		// Default: 7d
		Filters: []offchain.Filter{
			{
				Match: offchain.Match{
					Channel:      utils.Ptr("security"),
					ProviderGuid: utils.Ptr(provider.String()),
				},
				PolicyAction: offchain.PolicyAction{
					Type:            offchain.PolicyTypeTimestamp,
					RuleGroup:       offchain.RuleGroupingGlobal,
					RetentionPeriod: types.MarshalledDuration(time.Hour * 24 * 90),
				},
			},
			{
				Match: offchain.Match{
					EventId: utils.Ptr(events.EventId(12345)),
				},
				PolicyAction: offchain.PolicyAction{
					Type:            offchain.PolicyTypeTimestamp,
					RuleGroup:       offchain.RuleGroupingGlobal,
					RetentionPeriod: types.MarshalledDuration(time.Hour * 24 * 60),
				},
			},
			{
				PolicyAction: offchain.PolicyAction{
					Type:            offchain.PolicyTypeTimestamp,
					RuleGroup:       offchain.RuleGroupingGlobal,
					RetentionPeriod: types.MarshalledDuration(time.Hour * 24 * 7),
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var testData []any
	var expectedHashes []events.EventHash
	counter := newCounter() // Use to generate IDs

	// Events within default retention period
	for i := 0; i < 30; i++ {
		eventTime := time.Now().Add(-time.Hour * 24 * 6)

		ev := generateStoredEvent(suite.T(), counter.Next(), eventTime, "p", ChannelSecurity, 1, nil)
		testData = append(testData, ev)
	}

	// Events outside of default retention period
	for i := 0; i < 30; i++ {
		eventTime := time.Now().Add(-time.Hour * 24 * 8)

		ev := generateStoredEvent(suite.T(), counter.Next(), eventTime, "p", ChannelSecurity, 1, nil)
		testData = append(testData, ev)
		expectedHashes = append(expectedHashes, ev.Metadata.EventId)
	}

	// channel = security & event_id = 12345 (retained due to event_id)
	for i := 0; i < 30; i++ {
		eventTime := time.Now().Add(-time.Hour * 24 * 59)

		ev := generateStoredEvent(suite.T(), counter.Next(), eventTime, "p", ChannelSecurity, 12345, nil)
		testData = append(testData, ev)
	}

	// event_id = 12345, outside of retention period
	for i := 0; i < 30; i++ {
		eventTime := time.Now().Add(-time.Hour * 24 * 61)

		ev := generateStoredEvent(suite.T(), counter.Next(), eventTime, "p", ChannelSecurity, 12345, nil)
		testData = append(testData, ev)
		expectedHashes = append(expectedHashes, ev.Metadata.EventId)
	}

	// channel = security & provider = ...
	for i := 0; i < 30; i++ {
		eventTime := time.Now().Add(-time.Hour * 24 * 89)

		ev := generateStoredEvent(suite.T(), counter.Next(), eventTime, "p", ChannelSecurity, 1, provider)
		testData = append(testData, ev)
	}

	// channel = system & provider = ... (not kept)
	for i := 0; i < 30; i++ {
		eventTime := time.Now().Add(-time.Hour * 24 * 89)

		ev := generateStoredEvent(suite.T(), counter.Next(), eventTime, "p", ChannelSystem, 1, provider)
		testData = append(testData, ev)
		expectedHashes = append(expectedHashes, ev.Metadata.EventId)
	}

	// channel = security & provider = ... outside of retention period
	for i := 0; i < 30; i++ {
		eventTime := time.Now().Add(-time.Hour * 24 * 100)

		ev := generateStoredEvent(suite.T(), counter.Next(), eventTime, "p", ChannelSecurity, 1, provider)
		testData = append(testData, ev)
		expectedHashes = append(expectedHashes, ev.Metadata.EventId)
	}

	_, err := suite.Collection().InsertMany(ctx, testData)
	suite.Require().NoErrorf(err, "Could not insert test data: %s", err)

	aggregate, err := buildAggregate(policy)
	suite.Require().NoErrorf(err, "Could not build aggregate: %s", err)

	cursor, err := suite.Collection().Aggregate(ctx, aggregate)
	suite.Require().NoErrorf(err, "Could not aggregate: %s", err)

	requireHashes(suite.T(), cursor, expectedHashes)
}

func (suite *RetentionTestSuite) TestRetainMatchMultipleTimestamp() {
	policy := offchain.RetentionPolicy{
		Filters: []offchain.Filter{
			{
				Match: offchain.Match{
					Channel: utils.Ptr(ChannelSecurity),
					EventId: utils.Ptr(events.EventId(12345)),
				},
				PolicyAction: offchain.PolicyAction{
					Type:            offchain.PolicyTypeTimestamp,
					RetentionPeriod: types.MarshalledDuration(time.Hour * 24 * 90),
				},
			},
			{
				PolicyAction: offchain.PolicyAction{
					Type:            offchain.PolicyTypeTimestamp,
					RetentionPeriod: types.MarshalledDuration(time.Hour * 24 * 30),
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var testData []any
	var expectedHashes []events.EventHash
	for i := 0; i < 100; i++ {
		eventTime := time.Now().Add(-time.Hour * 24 * 60)

		ev := generateStoredEvent(suite.T(), i, eventTime, "p", ChannelSecurity, 12345, nil)
		testData = append(testData, ev)
	}

	for i := 100; i < 130; i++ {
		eventTime := time.Now().Add(-time.Hour * 24 * 60)

		ev := generateStoredEvent(suite.T(), i, eventTime, "p", ChannelSecurity, 1, nil)
		testData = append(testData, ev)
		expectedHashes = append(expectedHashes, ev.Metadata.EventId)
	}

	for i := 130; i < 160; i++ {
		eventTime := time.Now().Add(-time.Hour * 24 * 60)

		ev := generateStoredEvent(suite.T(), i, eventTime, "p", ChannelSystem, 12345, nil)
		testData = append(testData, ev)
		expectedHashes = append(expectedHashes, ev.Metadata.EventId)
	}

	_, err := suite.Collection().InsertMany(ctx, testData)
	suite.Require().NoErrorf(err, "Could not insert test data: %s", err)

	aggregate, err := buildAggregate(policy)
	suite.Require().NoErrorf(err, "Could not build aggregate: %s", err)

	cursor, err := suite.Collection().Aggregate(ctx, aggregate)
	suite.Require().NoErrorf(err, "Could not aggregate: %s", err)

	requireHashes(suite.T(), cursor, expectedHashes)
}

func (suite *RetentionTestSuite) TestRetainChannelVolume() {
	policy := offchain.RetentionPolicy{
		Filters: []offchain.Filter{
			{
				Match: offchain.Match{
					Channel: utils.Ptr("SyStEm"), // Test case insensitivity
				},
				PolicyAction: offchain.PolicyAction{
					Type:            offchain.PolicyTypeTimestamp,
					RuleGroup:       offchain.RuleGroupingGlobal,
					RetentionPeriod: types.MarshalledDuration(time.Hour * 24 * 90),
				},
			},
			{
				PolicyAction: offchain.PolicyAction{
					Type:      offchain.PolicyTypeCount,
					RuleGroup: offchain.RuleGroupingGlobal,
					Volume:    20,
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var testData []any
	var hashes []events.EventHash
	for i := 0; i < 100; i++ {
		eventTime := time.Now().Add(-time.Hour*24*60 - time.Minute*time.Duration(i))

		channel := ChannelSecurity
		if i < 50 {
			channel = ChannelSystem
		}

		ev := generateStoredEvent(suite.T(), i, eventTime, "principal", channel, i, nil)
		testData = append(testData, ev)
		hashes = append(hashes, ev.Metadata.EventId)
	}

	_, err := suite.Collection().InsertMany(ctx, testData)
	suite.Require().NoErrorf(err, "Could not insert test data: %s", err)

	aggregate, err := buildAggregate(policy)
	suite.Require().NoErrorf(err, "Could not build aggregate: %s", err)

	cursor, err := suite.Collection().Aggregate(ctx, aggregate, options.Aggregate().SetCollation(collationCaseInsensitive))
	suite.Require().NoErrorf(err, "Could not aggregate: %s", err)

	// Delete 30 oldest events with "Security" channel - "System" channel is excluded, and there is capacity
	// for 20 more non-matching events.
	expected := hashes[70:]
	requireHashes(suite.T(), cursor, expected)
}

func (suite *RetentionTestSuite) TestRetainAdvancedVolumeGroupedAndMatch() {
	policy := offchain.RetentionPolicy{
		Filters: []offchain.Filter{
			{
				PolicyAction: offchain.PolicyAction{
					Type:      offchain.PolicyTypeCount,
					RuleGroup: offchain.RuleGroupingPrincipal,
					Volume:    3, // 3 events per principal
				},
			},
			{
				Match: offchain.Match{
					Channel:      utils.Ptr("security"),
					EventId:      utils.Ptr(events.EventId(12345)),
					ProviderGuid: nil,
				},
				PolicyAction: offchain.PolicyAction{
					Type:            offchain.PolicyTypeTimestamp,
					RetentionPeriod: types.MarshalledDuration(time.Hour * 24 * 90),
				},
			},
			{
				PolicyAction: offchain.PolicyAction{
					Type:            offchain.PolicyTypeTimestamp,
					RetentionPeriod: types.MarshalledDuration(time.Hour * 24 * 30),
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var testData []any
	var expectedHashes []events.EventHash
	principals := []string{"p1", "p2", "p3"}

	// Security events that *do not* match the required event ID, outside of the default retention period
	for i := 0; i < 30; i++ {
		eventTime := time.Now().Add(-time.Hour*24*60 - time.Minute*time.Duration(i))

		ev := generateStoredEvent(suite.T(), i, eventTime, principals[i%3], ChannelSecurity, i, nil)
		testData = append(testData, ev)
		expectedHashes = append(expectedHashes, ev.Metadata.EventId)
	}

	// Security events with the required event ID, to be kept
	for i := 30; i < 60; i++ {
		eventTime := time.Now().Add(-time.Hour*24*60 - time.Minute*time.Duration(i))

		ev := generateStoredEvent(suite.T(), i, eventTime, principals[i%3], ChannelSecurity, 12345, nil)
		testData = append(testData, ev)
	}

	// System events, but with the required event ID, to be removed as the channel does not match
	for i := 60; i < 90; i++ {
		eventTime := time.Now().Add(-time.Hour*24*60 - time.Minute*time.Duration(i))

		ev := generateStoredEvent(suite.T(), i, eventTime, principals[i%3], ChannelSystem, 12345, nil)
		testData = append(testData, ev)
		expectedHashes = append(expectedHashes, ev.Metadata.EventId)
	}

	// Events inside 30 days
	for i := 90; i < 120; i++ {
		eventTime := time.Now().Add(-time.Hour * 24 * 20)

		ev := generateStoredEvent(suite.T(), i, eventTime, "p", ChannelSecurity, i, nil)
		testData = append(testData, ev)
	}

	// 1 security event with required day older than 90 days per principal
	for i, principal := range principals {
		eventTime := time.Now().Add(-time.Hour * 24 * 92)

		ev := generateStoredEvent(suite.T(), 120+i, eventTime, principal, ChannelSecurity, 12345, nil)
		testData = append(testData, ev)
		expectedHashes = append(expectedHashes, ev.Metadata.EventId)
	}

	// Other events
	for i := 120 + len(principals); i < 150+len(principals); i++ {
		eventTime := time.Now().Add(-time.Hour*24*50 - time.Minute*time.Duration(i))

		ev := generateStoredEvent(suite.T(), i, eventTime, principals[i%3], ChannelSecurity, i, nil)
		testData = append(testData, ev)

		if i >= 120+len(principals)+len(principals)*3 {
			expectedHashes = append(expectedHashes, ev.Metadata.EventId)
		}
	}

	_, err := suite.Collection().InsertMany(ctx, testData)
	suite.Require().NoErrorf(err, "Could not insert test data: %s", err)

	aggregate, err := buildAggregate(policy)
	suite.Require().NoErrorf(err, "Could not build aggregate: %s", err)

	cursor, err := suite.Collection().Aggregate(ctx, aggregate, options.Aggregate().SetCollation(collationCaseInsensitive))
	suite.Require().NoErrorf(err, "Could not aggregate: %s", err)

	requireHashes(suite.T(), cursor, expectedHashes)
}

func generateStoredEvent(t *testing.T, i int, eventTime time.Time, principal string, channel string, eventId int, provider *events.Guid) events.StoredEvent {
	t.Helper()

	return events.StoredEvent{
		EventWithData: events.EventWithData{
			Event: events.Event{
				System: events.System{
					Provider: events.Provider{
						Name: utils.Ptr("Test-Provider"),
						Guid: provider,
					},
					EventId: events.EventId(eventId),
					Channel: channel,
				},
			},
			EventData: events.EventData{
				{
					Name:  utils.Ptr("data-point"),
					Value: utils.Ptr("data-value"),
				},
			},
		},
		Metadata: events.Metadata{
			EventId:      generateHash(t, i),
			ReceivedTime: eventTime,
			Principal:    identity.Principal(principal),
		},
		TxHash: generateHash(t, i),
	}
}

func requireHashes(t *testing.T, cursor *mongo.Cursor, expected []events.EventHash) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var results []policyScanResult
	require.NoError(t, cursor.All(ctx, &results))

	require.Len(t, results, 1)

	// Provide better debugging information
	if !assert.ElementsMatch(t, expected, results[0].Events) {
		for i, hash := range expected {
			t.Logf("Expected[%d]: %s", i, hash)
		}

		for i, hash := range results[0].Events {
			t.Logf("Actual[%d]: %s", i, hash)
		}

		t.FailNow()
	}
}

func generateHash(t *testing.T, i int) events.TxHash {
	t.Helper()

	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, uint64(i))

	digest := sha256.New()
	_, err := digest.Write(data)
	require.NoError(t, err)

	return digest.Sum(nil)
}

type intCounter struct {
	current int
}

func newCounter() intCounter {
	return intCounter{}
}

func (c *intCounter) Get() int {
	return c.current
}

func (c *intCounter) Next() int {
	c.current++
	return c.current
}
