package accounts

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/codex-team/hawk.collector/pkg/periodic"
	log "github.com/sirupsen/logrus"
)

const collectionWithTokens = "projects"
const tokenFieldName = "token"
const contextTimeout = 5 * time.Second

func (client *AccountsMongoDBClient) Run(TokenUpdatePeriod time.Duration) {
	done := make(chan struct{})
	go periodic.RunPeriodically(client.UpdateTokenCache, TokenUpdatePeriod, done)
	// TODO: chan closing process
}

func (client *AccountsMongoDBClient) UpdateTokenCache() error {
	log.Debugf("Run UpdateCache for MongoDB tokens")

	ctx, _ := context.WithTimeout(context.Background(), contextTimeout)
	collection := client.mdb.Database(client.database).Collection(collectionWithTokens)
	tokens, err := collection.Distinct(ctx, tokenFieldName, bson.D{})
	if err != nil {
		log.Errorf("Cannot update cache: %s", err)
		return err
	}

	client.validTokens = make([]string, len(tokens))
	for _, token := range tokens {
		client.validTokens = append(client.validTokens, token.(string))
	}

	log.Debugf("Cache for MongoDB tokens successfully updates with %d tokens", len(client.validTokens))

	return nil
}
