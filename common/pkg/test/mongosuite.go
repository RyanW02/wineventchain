package test

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"testing"
	"time"
)

type MongoSuite struct {
	suite.Suite

	databaseName   string
	collectionName string

	pool     *dockertest.Pool
	resource *dockertest.Resource

	client     *mongo.Client
	database   *mongo.Database
	collection *mongo.Collection
}

const MongoImageTag = "7"

func (suite *MongoSuite) Collection() *mongo.Collection {
	return suite.collection
}

func (suite *MongoSuite) SetupSuite() {
	suite.databaseName = fmt.Sprintf("test_db_%s", uuid.New())
	suite.collectionName = fmt.Sprintf("test_col_%s", uuid.New())

	suite.T().Log("Connecting to Docker...")
	pool, err := dockertest.NewPool("")
	suite.Require().NoErrorf(err, "Could not connect to Docker: %s", err)

	pool.MaxWait = time.Minute * 3

	suite.Require().NoErrorf(pool.Client.Ping(), "Could not ping Docker: %s", err)

	suite.T().Log("Connected to Docker, starting MongoDB container...")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	client, resource, err := startMongoDB(ctx, suite.T(), pool)
	suite.Require().NoErrorf(err, "Could not start MongoDB: %s", err)

	suite.pool = pool
	suite.resource = resource

	suite.client = client
	suite.database = client.Database(suite.databaseName)
	suite.collection = suite.database.Collection(suite.collectionName)

	_, err = suite.collection.DeleteMany(context.Background(), bson.D{})
	suite.Require().NoErrorf(err, "Could not delete documents from collection")
}

func startMongoDB(ctx context.Context, t *testing.T, pool *dockertest.Pool) (*mongo.Client, *dockertest.Resource, error) {
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "mongo",
		Tag:        MongoImageTag,
		Env: []string{
			"MONGO_INITDB_ROOT_USERNAME=root",
			"MONGO_INITDB_ROOT_PASSWORD=password",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		return nil, nil, err
	}

	t.Log("MongoDB container started, waiting for it to be ready...")

	var client *mongo.Client
	if err := pool.Retry(func() error {
		var err error
		client, err = mongo.Connect(
			ctx,
			options.Client().ApplyURI(
				fmt.Sprintf("mongodb://root:password@localhost:%s", resource.GetPort("27017/tcp")),
			),
		)
		if err != nil {
			return err
		}

		return client.Ping(ctx, nil)
	}); err != nil {
		return nil, nil, err
	}

	t.Log("MongoDB container is ready")

	return client, resource, nil
}

func (suite *MongoSuite) SetupTest() {
	suite.collection = suite.database.Collection(suite.collectionName)
}

func (suite *MongoSuite) TearDownTest() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	suite.Require().NoErrorf(suite.collection.Drop(ctx), "Failed to drop collection after test")
}

func (suite *MongoSuite) TearDownSuite() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	suite.Assert().NoErrorf(suite.database.Drop(ctx), "Could not drop database")
	cancel()

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	suite.Require().NoErrorf(suite.client.Disconnect(ctx), "Could not disconnect from MongoDB")

	suite.Require().NoErrorf(suite.pool.Purge(suite.resource), "Could not purge Docker resource")
}

func (suite *MongoSuite) TestDatabaseOnline() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	suite.Require().NoErrorf(suite.client.Ping(ctx, nil), "Could not ping MongoDB")
}
