package accounts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"go.mongodb.org/mongo-driver/bson"

	log "github.com/sirupsen/logrus"
)

const projectsCollectionName = "projects"
const contextTimeout = 5 * time.Second

type acountToken struct {
	IntegrationId string `json:"integrationId"`
	Secret        string `json:"secret"`
}

type accountProject struct {
	ProjectID primitive.ObjectID `bson:"_id"`
	Token     string             `bson:"token"`
}

func (client *AccountsMongoDBClient) UpdateTokenCache() error {
	log.Debugf("Run UpdateCache for MongoDB tokens")

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()
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
		integrationSecret, err := DecodeToken(project.Token)
		if err == nil {
			client.ValidTokens[integrationSecret] = project.ProjectID.Hex()
		} else {
			log.Errorf("Integration token %s is invalid: %s", project.Token, err)
		}
	}

	log.Debugf("Cache for MongoDB tokens successfully updates with %d tokens", len(client.ValidTokens))
	log.Tracef("Current token cache state: %s", client.ValidTokens)

	return nil
}

// decodeToken decodes token from base64 to integrationId + secret
func DecodeToken(token string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", err
	}
	var data acountToken
	err = json.Unmarshal(decoded, &data)
	if err != nil {
		return "", err
	}

	integrationId := strings.ReplaceAll(data.IntegrationId, "-", "")
	secret := strings.ReplaceAll(data.Secret, "-", "")
	return integrationId + secret, nil
}
