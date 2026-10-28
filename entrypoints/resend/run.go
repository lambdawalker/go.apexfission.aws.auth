package entrypoint

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/lambdawalker/go.apexfission.aws.auth/api"
	"github.com/lambdawalker/go.apexfission.aws.auth/entrypoints/internal/bootstrap"
)

func Run() {
	handler := api.Handler{Service: bootstrap.Service()}
	lambda.Start(handler.Resend)
}
