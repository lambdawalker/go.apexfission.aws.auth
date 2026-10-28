package entrypoint

import (
	"context"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/lambdawalker/go.apexfission.aws.auth/awscapture"
	"github.com/lambdawalker/go.apexfission.aws.auth/captureapi"
	"github.com/lambdawalker/go.apexfission.aws.auth/envreset"
	"log"
	"os"
)

func Run() {
	s, store, _, e := awscapture.New(context.Background())
	if e != nil {
		log.Fatal("capture initialization failed")
	}
	if os.Getenv("RESET_TABLE") == "" {
		log.Fatal("missing reset gate configuration")
	}
	gate := envreset.DynamoStore{DB: store.DB, Table: os.Getenv("RESET_TABLE")}
	lambda.Start(captureapi.Handler{Service: s, CheckToken: gate.CheckToken}.Handle)
}
