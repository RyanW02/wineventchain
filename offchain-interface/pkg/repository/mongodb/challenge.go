package mongodb

import (
	"context"
	"errors"
	"github.com/RyanW02/wineventchain/common/pkg/types/identity"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/repository"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"time"
)

type MongoChallengeRepository struct {
	logger     *zap.Logger
	collection *mongo.Collection
}

type challengeRecord struct {
	Principal identity.Principal `bson:"principal"`
	Challenge []byte             `bson:"challenge"`
	Timestamp time.Time          `bson:"timestamp"`
}

func NewMongoChallengeRepository(logger *zap.Logger, db *mongo.Database) *MongoChallengeRepository {
	return &MongoChallengeRepository{
		logger:     logger,
		collection: db.Collection("challenges"),
	}
}

var (
	_ repository.ChallengeRepository = (*MongoChallengeRepository)(nil)
	_ mongoCollection                = (*MongoChallengeRepository)(nil)
)

func (m *MongoChallengeRepository) InitSchema(ctx context.Context) error {
	_, err := m.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{"principal", 1}},
	})

	return err
}

func (m *MongoChallengeRepository) AddChallenge(ctx context.Context, principal identity.Principal, challenge []byte) error {
	_, err := m.collection.InsertOne(ctx, challengeRecord{
		Principal: principal,
		Challenge: challenge,
		Timestamp: time.Now(),
	})
	return err
}

func (m *MongoChallengeRepository) GetAndRemoveChallenge(
	ctx context.Context,
	principal identity.Principal,
	challenge []byte,
	challengeLifetime time.Duration,
) (bool, error) {
	filter := bson.M{
		"principal": principal,
		"challenge": challenge,
	}

	res := m.collection.FindOneAndDelete(ctx, filter)
	if err := res.Err(); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		} else {
			return false, err
		}
	}

	var record challengeRecord
	if err := res.Decode(&record); err != nil {
		return false, err
	}

	isValid := record.Timestamp.After(time.Now().Add(-challengeLifetime))
	return isValid, nil
}

type challengeRecordWithId struct {
	Id interface{} `bson:"_id"`
	challengeRecord
}

func (m *MongoChallengeRepository) DropExpiredChallenges(ctx context.Context, challengeLifetime time.Duration) error {
	cursor, err := m.collection.Find(ctx, bson.M{
		"timestamp": bson.M{
			"$lt": time.Now().Add(-challengeLifetime),
		},
	})
	if err != nil {
		return err
	}

	var results []challengeRecordWithId
	if err := cursor.All(ctx, &results); err != nil {
		return err
	}

	var ids []interface{}
	for _, result := range results {
		ids = append(ids, result.Id)
	}

	if len(ids) == 0 {
		m.logger.Debug("no expired challenges to remove")
		return nil
	}

	m.logger.Info("removing expired challenges", zap.Int("count", len(ids)))

	_, err = m.collection.DeleteMany(ctx, bson.M{
		"_id": bson.M{
			"$in": ids,
		},
	})
	return err
}
