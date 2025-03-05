package accounts

import (
	"context"
	"net/url"
	"path"
	"time"

	"go.mongodb.org/mongo-driver/mongo/readpref"

	"go.mongodb.org/mongo-driver/mongo/options"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

const connectionTimeout = 10 * time.Second
const pingTimeout = 2 * time.Second

type AccountsMongoDBClient struct {
	mdb         *mongo.Client
	ctx         context.Context
	database    string
	ValidTokens map[string]string
}

func New(connectionURI string) *AccountsMongoDBClient {
	uriPath, err := url.Parse(connectionURI)
	if err != nil {
		log.Fatalf("Incorrect MongoDB connection URI: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(connectionURI))
	if err != nil {
		log.Fatalf("MongoDB connect error: %s", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		log.Fatalf("MongoDB ping error: %s", err)
	}

	// Log connection URL without credentials
	uriPath.User = &url.Userinfo{}
	log.Infof("✓ MongoDB accounts client initialized via %s", connectionURI)

	return &AccountsMongoDBClient{
		mdb:      client,
		ctx:      ctx,
		database: path.Base(uriPath.Path),
	}
}

// CheckAvailability checks if mongodb is available
func (m *AccountsMongoDBClient) CheckAvailability() bool {
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	err := m.mdb.Ping(ctx, readpref.Primary())
	return err == nil
}
