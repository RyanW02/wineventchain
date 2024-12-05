package mongodb

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/RyanW02/wineventchain/common/pkg/types/events"
	"github.com/RyanW02/wineventchain/common/pkg/types/offchain"
	"github.com/RyanW02/wineventchain/common/pkg/utils"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/repository"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"strconv"
	"time"
)

const EventCollectionName = "events"

type MongoEventRepository struct {
	logger     *zap.Logger
	collection *mongo.Collection
}

// Compile-time type validation
var (
	_ repository.EventRepository = (*MongoEventRepository)(nil)
	_ mongoCollection            = (*MongoEventRepository)(nil)

	collationCaseInsensitive = &options.Collation{
		Locale:   "en",
		Strength: 1,
	}
)

func NewMongoEventRepository(logger *zap.Logger, db *mongo.Database) *MongoEventRepository {
	return &MongoEventRepository{
		logger:     logger,
		collection: db.Collection(EventCollectionName),
	}
}

func (m *MongoEventRepository) InitSchema(ctx context.Context) error {
	_, err := m.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		// Index metadata
		{
			Keys:    bson.D{{KeyEventId, 1}},
			Options: options.Index().SetUnique(true).SetCollation(collationCaseInsensitive),
		},
		{
			Keys: bson.M{KeyTimestamp: -1},
		},
		{
			Keys: bson.M{KeyPrincipal: 1},
		},
		// Index on key attributes
		{
			Keys:    bson.M{KeyChannel: 1},
			Options: options.Index().SetCollation(collationCaseInsensitive), // Case-insensitive
		},
		{
			Keys: bson.M{KeyEventTypeId: 1},
		},
		{
			Keys: bson.M{KeyProvider: 1},
		},
	})

	return err
}

func (m *MongoEventRepository) GetEventById(ctx context.Context, id events.EventHash) (events.StoredEvent, bool, error) {
	var event events.StoredEvent

	filter := bson.D{{"metadata.event_id", id}}
	if err := m.collection.FindOne(ctx, filter).Decode(&event); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return events.StoredEvent{}, false, nil
		} else {
			return events.StoredEvent{}, false, err
		}
	}

	return event, true, nil
}

func (m *MongoEventRepository) GetEventsById(ctx context.Context, ids []events.EventHash) ([]events.StoredEvent, error) {
	filter := bson.M{"metadata.event_id": bson.M{"$in": ids}}
	cursor, err := m.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}

	var events []events.StoredEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}

	return events, nil
}

func (m *MongoEventRepository) GetEventByTx(ctx context.Context, txHash []byte) (events.StoredEvent, bool, error) {
	var event events.StoredEvent

	filter := bson.D{{"tx_hash", events.TxHash(txHash)}}
	if err := m.collection.FindOne(ctx, filter).Decode(&event); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return events.StoredEvent{}, false, nil
		} else {
			return events.StoredEvent{}, false, err
		}
	}

	return event, true, nil
}

type recordWithId[T any] struct {
	Id     primitive.ObjectID `bson:"_id"`
	Record T                  `bson:"inline"`
}

func (m *MongoEventRepository) SearchEvents(ctx context.Context, filters []repository.Filter, limit, page int, first *events.EventHash) ([]events.StoredEvent, error) {
	// Lookup timestamp of [after] event
	var firstObjectId *primitive.ObjectID
	if first != nil {
		var event recordWithId[events.StoredEvent]

		filter := bson.D{{"metadata.event_id", *first}}
		if err := m.collection.FindOne(ctx, filter).Decode(&event); err != nil {
			// If the event is not found, return the first page of results
			if !errors.Is(err, mongo.ErrNoDocuments) {
				return nil, err
			}
		} else {
			firstObjectId = &event.Id
		}
	}

	filter, err := buildFilter(filters, firstObjectId)
	if err != nil {
		return nil, err
	}

	opts := options.Find().
		SetSort(bson.D{{KeyTimestamp, -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(page * limit)).
		SetCollation(collationCaseInsensitive)

	cursor, err := m.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}

	var events []events.StoredEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}

	return events, nil
}

func (m *MongoEventRepository) EventCount(ctx context.Context) (int, error) {
	count, err := m.collection.CountDocuments(ctx, bson.D{}, nil)
	return int(count), err
}

func (m *MongoEventRepository) Store(ctx context.Context, event events.StoredEvent) error {
	if _, err := m.collection.InsertOne(ctx, event); err != nil {
		// Check for duplicate key error
		var ex mongo.WriteException
		if errors.As(err, &ex) && ex.WriteErrors[0].Code == 11000 {
			return repository.ErrEventAlreadyStored
		} else {
			return err
		}
	}

	return nil
}

type policyScanResult struct {
	Events []events.EventHash `bson:"events"`
}

func (m *MongoEventRepository) DropExpiredEvents(ctx context.Context, policy offchain.RetentionPolicy) error {
	if err := policy.Validate(); err != nil {
		return err
	}

	// Build the aggregation pipeline
	aggregate, err := buildAggregate(policy)
	if err != nil {
		return err
	}

	m.logger.Info("Fetching events outside of retention policy to drop")
	m.logger.Debug("Built aggregation pipeline", zap.Any("pipeline", aggregate))

	// Execute the aggregation pipeline to get a list of event IDs to delete
	cursor, err := m.collection.Aggregate(ctx, aggregate)
	if err != nil {
		return err
	}

	var results []policyScanResult
	if err := cursor.All(ctx, &results); err != nil {
		return err
	}

	// If no events to be deleted, the results will be empty, *not* [events:[]]
	if len(results) == 0 {
		m.logger.Info("No events to drop")
		return nil
	}

	m.logger.Info("Found events outside of retention policy", zap.Int("count", len(results[0].Events)))

	// Delete the events
	filter := bson.M{"metadata.event_id": bson.M{"$in": results[0].Events}}
	res, err := m.collection.DeleteMany(ctx, filter)
	if err != nil {
		return err
	}

	m.logger.Info(
		"Deleted events outside of retention policy",
		zap.Int64("deleted_count", res.DeletedCount),
		zap.Int("expected_count", len(results[0].Events)),
	)

	return nil
}

func buildFilter(filters []repository.Filter, first *primitive.ObjectID) (bson.M, error) {
	filter := bson.M{}

	if first != nil {
		filter["_id"] = bson.M{"$lte": *first}
	}

	for _, f := range filters {
		switch f.Property {
		case repository.PropertyEventId:
			if f.Operator != repository.OperatorEqual {
				return nil, fmt.Errorf("%w: invalid operator for event ID", repository.ErrInvalidFilter)
			}

			eventId, err := hex.DecodeString(f.Value)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid event ID", repository.ErrInvalidFilter)
			}

			filter["metadata.event_id"] = events.EventHash(eventId)
		case repository.PropertyTxHash:
			if f.Operator != repository.OperatorEqual {
				return nil, fmt.Errorf("%w: invalid operator for tx hash", repository.ErrInvalidFilter)
			}

			txHash, err := hex.DecodeString(f.Value)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid tx hash", repository.ErrInvalidFilter)
			}

			filter["tx_hash"] = events.TxHash(txHash)
		case repository.PropertyPrincipal:
			if f.Operator != repository.OperatorEqual {
				return nil, fmt.Errorf("%w: invalid operator for principal", repository.ErrInvalidFilter)
			}

			filter["metadata.principal"] = f.Value
		case repository.PropertyEventType:
			if f.Operator != repository.OperatorEqual {
				return nil, fmt.Errorf("%w: invalid operator for event type ID", repository.ErrInvalidFilter)
			}

			eventId, err := strconv.Atoi(f.Value)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid event type ID", repository.ErrInvalidFilter)
			}

			filter["event.event.system.event_id"] = eventId
		case repository.PropertyTimestamp:
			timestamp, err := time.Parse(utils.HtmlDateTimeFormat, f.Value)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid timestamp", repository.ErrInvalidFilter)
			}

			dateTime := primitive.NewDateTimeFromTime(timestamp)

			if filter["metadata.received_time"] == nil {
				filter["metadata.received_time"] = bson.M{}
			}

			if f.Operator == repository.OperatorBefore {
				filter["metadata.received_time"].(bson.M)["$lt"] = dateTime
			} else if f.Operator == repository.OperatorAfter {
				filter["metadata.received_time"].(bson.M)["$gt"] = dateTime
			} else {
				return nil, fmt.Errorf("%w: invalid operator for timestamp", repository.ErrInvalidFilter)
			}
		case repository.ProviderName:
			if f.Operator != repository.OperatorEqual {
				return nil, fmt.Errorf("%w: invalid operator for provider", repository.ErrInvalidFilter)
			}

			filter["event.event.system.provider.name"] = f.Value
		case repository.PropertyProviderGuid:
			if f.Operator != repository.OperatorEqual {
				return nil, fmt.Errorf("%w: invalid operator for provider", repository.ErrInvalidFilter)
			}

			uuid, err := uuid.Parse(f.Value)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid provider GUID", repository.ErrInvalidFilter)
			}

			filter["event.event.system.provider.guid"] = events.Guid(uuid)
		case repository.PropertyCorrelation:
			if f.Operator != repository.OperatorEqual {
				return nil, fmt.Errorf("%w: invalid operator for provider", repository.ErrInvalidFilter)
			}

			uuid, err := uuid.Parse(f.Value)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid provider GUID", repository.ErrInvalidFilter)
			}

			filter["event.event.system.correlation.activity_id"] = events.Guid(uuid)
		case repository.PropertyChannel:
			if f.Operator != repository.OperatorEqual {
				return nil, fmt.Errorf("%w: invalid operator for channel", repository.ErrInvalidFilter)
			}

			filter["event.event.system.channel"] = f.Value
		}
	}

	return filter, nil
}
