package bootstrap

import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/lambdawalker/go.apexfission.aws.auth/awsemail"
	"github.com/lambdawalker/go.apexfission.aws.auth/awsidentity"
	"github.com/lambdawalker/go.apexfission.aws.auth/awsproofkey"
	"github.com/lambdawalker/go.apexfission.aws.auth/awsstore"
	"github.com/lambdawalker/go.apexfission.aws.auth/email"
)

// Service configures the common proof store and identity clients for each API Lambda.
func Service() *email.Service {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatal("AWS config unavailable")
	}
	service, err := newService(context.Background(), cfg, os.Getenv, secretsmanager.NewFromConfig(cfg))
	if err != nil {
		log.Fatal(err)
	}
	return service
}

// newService resolves the proof key once before handlers begin serving requests.
func newService(ctx context.Context, cfg aws.Config, getenv func(string) string, secrets awsproofkey.GetSecretValueAPI) (*email.Service, error) {
	table := getenv("TABLE_NAME")
	pool := getenv("POOL_ID")
	client := getenv("CLIENT_ID")
	origin := getenv("APP_ORIGIN")
	sender := getenv("SENDER_ADDRESS")
	if table == "" || pool == "" || client == "" || origin == "" || sender == "" {
		return nil, errors.New("missing required configuration")
	}
	key, err := awsproofkey.Load(ctx, secrets, getenv("PROOF_KEY_SECRET_ARN"), getenv("PROOF_KEY_SECRET_VERSION"))
	if err != nil {
		return nil, err
	}
	db := dynamodb.NewFromConfig(cfg)
	return &email.Service{
		Store:       awsstore.Store{DB: db, Table: table},
		Identity:    awsidentity.Identity{Cognito: cognitoidentityprovider.NewFromConfig(cfg), DB: db, Table: table, PoolID: pool, ClientID: client},
		Sender:      awsemail.Sender{Client: sesv2.NewFromConfig(cfg), From: sender},
		Key:         key,
		Origin:      origin,
		Diagnostics: getenv("DIAGNOSTIC_MODE") == "true",
	}, nil
}
