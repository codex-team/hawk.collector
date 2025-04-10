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
	"go.mongodb.org/mongo-driver/mongo"
)

const projectsCollectionName = "projects"
const workspacesCollectionName = "workspaces"
const plansCollectionName = "plans"
const contextTimeout = 5 * time.Second

type acountToken struct {
	IntegrationId string `json:"integrationId"`
	Secret        string `json:"secret"`
}

type accountProject struct {
	ProjectID         primitive.ObjectID `bson:"_id"`
	Token             string             `bson:"token"`
	WorkspaceID       primitive.ObjectID `bson:"workspaceId"`
	RateLimitSettings rateLimitSettings  `bson:"rateLimitSettings"`
}

type rateLimitSettings struct {
	EventsLimit  int64 `bson:"N"`
	EventsPeriod int64 `bson:"T"`
}

type tariffPlan struct {
	PlanID            primitive.ObjectID `bson:"_id"`
	RateLimitSettings rateLimitSettings  `bson:"rateLimitSettings"`
}

type accountWorkspace struct {
	WorkspaceID       primitive.ObjectID `bson:"_id"`
	TariffPlan        tariffPlan         `bson:"plan"`
	RateLimitSettings rateLimitSettings  `bson:"rateLimitSettings"`
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

	// Create a temporary map instead of directly modifying client.ValidTokens
	validTokensTmp := make(map[string]string)
	
	for _, project := range projects {
		integrationSecret, err := DecodeToken(project.Token)
		if err == nil {
			validTokensTmp[integrationSecret] = project.ProjectID.Hex()
		} else {
			log.Errorf("Integration token %s is invalid: %s", project.Token, err)
		}
	}
	
	// Atomically replace the map reference
	client.ValidTokens = validTokensTmp

	log.Debugf("Cache for MongoDB tokens successfully updates with %d tokens", len(client.ValidTokens))
	log.Tracef("Current token cache state: %s", client.ValidTokens)

	return nil
}

func (client *AccountsMongoDBClient) UpdateProjectsLimitsCache() error {
	log.Debugf("Run UpdateCache for MongoDB projects limits")

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	// Get workspaces with their plans using aggregation pipeline
	workspacesCollection := client.mdb.Database(client.database).Collection(workspacesCollectionName)
	pipeline := mongo.Pipeline{
		{
			{Key: "$lookup", Value: bson.D{
				{Key: "from", Value: plansCollectionName},
				{Key: "localField", Value: "tariffPlanId"},
				{Key: "foreignField", Value: "_id"},
				{Key: "as", Value: "plan"},
			}},
		},
		{
			{Key: "$unwind", Value: "$plan"},
		},
	}

	workspacesCursor, err := workspacesCollection.Aggregate(ctx, pipeline)
	if err != nil {
		log.Errorf("Cannot execute aggregation for workspaces and plans: %s", err)
		return err
	}

	var workspaces []accountWorkspace
	if err = workspacesCursor.All(ctx, &workspaces); err != nil {
		log.Errorf("Cannot decode aggregation results: %s", err)
		return err
	}

	// Get all projects
	projectsCollection := client.mdb.Database(client.database).Collection(projectsCollectionName)
	cursor, err := projectsCollection.Find(ctx, bson.D{})
	if err != nil {
		log.Errorf("Cannot create cursor in %s collection for cache update: %s", projectsCollectionName, err)
		return err
	}

	var projects []accountProject
	if err = cursor.All(ctx, &projects); err != nil {
		log.Errorf("Cannot decode data in %s collection for cache update: %s", projectsCollectionName, err)
		return err
	}

	// Create workspace lookup map for quick access
	workspaceMap := make(map[string]accountWorkspace)
	for _, workspace := range workspaces {
		workspaceMap[workspace.WorkspaceID.Hex()] = workspace
	}

	// Create a temporary map instead of directly modifying client.ProjectLimits
	projectLimitsTmp := make(map[string]rateLimitSettings)

	// Process each project applying the priority rules
	for _, project := range projects {
		projectID := project.ProjectID.Hex()
		var finalLimits rateLimitSettings

		log.Tracef("Project with id %s and limits %+v", projectID, project.RateLimitSettings)

		if workspace, exists := workspaceMap[project.WorkspaceID.Hex()]; exists {
			finalLimits = workspace.TariffPlan.RateLimitSettings

			if workspace.RateLimitSettings.EventsLimit > 0 {
				finalLimits.EventsLimit = workspace.RateLimitSettings.EventsLimit
			}
			if workspace.RateLimitSettings.EventsPeriod > 0 {
				finalLimits.EventsPeriod = workspace.RateLimitSettings.EventsPeriod
			}
		}

		if project.RateLimitSettings.EventsLimit > 0 {
			finalLimits.EventsLimit = project.RateLimitSettings.EventsLimit
		}
		if project.RateLimitSettings.EventsPeriod > 0 {
			finalLimits.EventsPeriod = project.RateLimitSettings.EventsPeriod
		}

		// Add to temporary map instead of client.ProjectLimits
		projectLimitsTmp[projectID] = finalLimits
	}

	// Atomically replace the map reference
	client.ProjectLimits = projectLimitsTmp

	log.Tracef("Current projects limits cache state: %+v", client.ProjectLimits)

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
