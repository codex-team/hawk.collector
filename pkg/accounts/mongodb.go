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

type AccountsMongoDBClient struct {
	mdb                         *mongo.Client
	ctx                         context.Context
	database                    string
	ValidTokens                 map[string]string
	AllowDeprecatedTokensFormat bool
}

func New(connectionURI string, allowDeprecatedTokensFormat bool) *AccountsMongoDBClient {
	uriPath, err := url.Parse(connectionURI)
	if err != nil {
		log.Fatalf("Incorrect MongoDB connection URI (%s): %s", connectionURI, err)
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

	log.Debugf("✓ MongoDB accounts client initialized via %s", connectionURI)

	return &AccountsMongoDBClient{
		mdb:                         client,
		ctx:                         ctx,
		database:                    path.Base(uriPath.Path),
		AllowDeprecatedTokensFormat: allowDeprecatedTokensFormat,
	}
}
