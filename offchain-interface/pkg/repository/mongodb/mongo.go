package mongodb

import (
	"context"
	"github.com/RyanW02/wineventchain/offchain-interface/pkg/repository"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"time"
)

type MongoRepository struct {
	database   *mongo.Database
	events     *MongoEventRepository
	challenges *MongoChallengeRepository
}

var _ repository.Repository = (*MongoRepository)(nil)

type mongoCollection interface {
	InitSchema(ctx context.Context) error
}

func NewMongoRepository(logger *zap.Logger, db *mongo.Database) *MongoRepository {
	return &MongoRepository{
		database:   db,
		events:     NewMongoEventRepository(logger, db),
		challenges: NewMongoChallengeRepository(logger, db),
	}
}

func (m *MongoRepository) InitSchema(ctx context.Context) error {
	group, ctx := errgroup.WithContext(ctx)

	cols := []mongoCollection{m.events, m.challenges}
	for _, col := range cols {
		col := col
		group.Go(func() error {
			return col.InitSchema(ctx)
		})
	}

	return group.Wait()
}

func (m *MongoRepository) Events() repository.EventRepository {
	return m.events
}

func (m *MongoRepository) Challenges() repository.ChallengeRepository {
	return m.challenges
}

func (m *MongoRepository) TestConnection() error {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	return m.database.Client().Ping(ctx, nil)
}
