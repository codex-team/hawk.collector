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
	TariffPlanID      primitive.ObjectID `bson:"tariffPlanId"`
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

	// Create a temporary map instead of directly modifying client.validTokens
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
	client.validTokens = validTokensTmp

	log.Debugf("Cache for MongoDB tokens successfully updates with %d tokens", len(client.validTokens))
	log.Tracef("Current token cache state: %s", client.validTokens)

	return nil
}

func (client *AccountsMongoDBClient) UpdateProjectsLimitsCache() error {
	log.Debugf("Run UpdateCache for MongoDB projects limits")

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	// Join plans in memory: $lookup would read plans once per workspace doc
	plansCollection := client.mdb.Database(client.database).Collection(plansCollectionName)
	plansCursor, err := plansCollection.Find(ctx, bson.D{})
	if err != nil {
		log.Errorf("Cannot create cursor in %s collection for cache update: %s", plansCollectionName, err)
		return err
	}

	var plans []tariffPlan
	if err = plansCursor.All(ctx, &plans); err != nil {
		log.Errorf("Cannot decode data in %s collection for cache update: %s", plansCollectionName, err)
		return err
	}

	plansMap := make(map[primitive.ObjectID]tariffPlan, len(plans))
	for _, plan := range plans {
		plansMap[plan.PlanID] = plan
	}

	workspacesCollection := client.mdb.Database(client.database).Collection(workspacesCollectionName)
	workspacesCursor, err := workspacesCollection.Find(ctx, bson.D{})
	if err != nil {
		log.Errorf("Cannot create cursor in %s collection for cache update: %s", workspacesCollectionName, err)
		return err
	}

	var workspaces []accountWorkspace
	if err = workspacesCursor.All(ctx, &workspaces); err != nil {
		log.Errorf("Cannot decode data in %s collection for cache update: %s", workspacesCollectionName, err)
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
	// Workspaces without a matching plan are skipped, as $unwind did before
	type workspaceWithPlan struct {
		workspace accountWorkspace
		plan      tariffPlan
	}
	workspaceMap := make(map[string]workspaceWithPlan, len(workspaces))
	for _, workspace := range workspaces {
		plan, ok := plansMap[workspace.TariffPlanID]
		if !ok {
			continue
		}
		workspaceMap[workspace.WorkspaceID.Hex()] = workspaceWithPlan{workspace: workspace, plan: plan}
	}

	// Create a temporary map instead of directly modifying client.projectLimits
	projectLimitsTmp := make(map[string]rateLimitSettings)

	// Process each project applying the priority rules
	for _, project := range projects {
		projectID := project.ProjectID.Hex()
		var finalLimits rateLimitSettings

		log.Tracef("Project with id %s and limits %+v", projectID, project.RateLimitSettings)

		if entry, exists := workspaceMap[project.WorkspaceID.Hex()]; exists {
			finalLimits = entry.plan.RateLimitSettings

			if entry.workspace.RateLimitSettings.EventsLimit > 0 {
				finalLimits.EventsLimit = entry.workspace.RateLimitSettings.EventsLimit
			}
			if entry.workspace.RateLimitSettings.EventsPeriod > 0 {
				finalLimits.EventsPeriod = entry.workspace.RateLimitSettings.EventsPeriod
			}
		}

		if project.RateLimitSettings.EventsLimit > 0 {
			finalLimits.EventsLimit = project.RateLimitSettings.EventsLimit
		}
		if project.RateLimitSettings.EventsPeriod > 0 {
			finalLimits.EventsPeriod = project.RateLimitSettings.EventsPeriod
		}

		// Add to temporary map instead of client.projectLimits
		projectLimitsTmp[projectID] = finalLimits
	}

	// Atomically replace the map reference
	client.projectLimits = projectLimitsTmp

	log.Tracef("Current projects limits cache state: %+v", client.projectLimits)

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
