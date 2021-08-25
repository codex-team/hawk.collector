package accounts

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"go.mongodb.org/mongo-driver/bson"

	log "github.com/sirupsen/logrus"
)

const projectsCollectionName = "projects"
const contextTimeout = 5 * time.Second

type accountProject struct {
	ProjectID primitive.ObjectID `bson:"_id"`
	Token     string             `bson:"token"`
}

func (client *AccountsMongoDBClient) UpdateTokenCache() error {
	log.Debugf("Run UpdateCache for MongoDB tokens")

	ctx, _ := context.WithTimeout(context.Background(), contextTimeout)
	collection := client.mdb.Database(client.database).Collection(projectsCollectionName)
	cursor, err := collection.Find(ctx, bson.D{})
	if err != nil {
		log.Errorf("Cannot create cursor in %s collection for cache update: %s", projectsCollectionName, err)
		return err
	}

	var projects []accountProject
	if err = cursor.All(ctx, &projects); err != nil {
		log.Errorf("Cannot decode data in %s collection for cache update: %s", projectsCollectionName, err)
		return err
	}

	client.ValidTokens = make(map[string]string)
	for _, project := range projects {
		client.ValidTokens[project.Token] = project.ProjectID.Hex()
	}

	log.Debugf("Cache for MongoDB tokens successfully updates with %d tokens", len(client.ValidTokens))
	log.Tracef("Current token cache state: %s", client.ValidTokens)

	return nil
}
