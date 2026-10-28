package main

import (
	"context"
	"encoding/base64"
	"github.com/aws/aws-lambda-go/lambda"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/lambdawalker/go.apexfission.aws.auth/admin"
	"github.com/lambdawalker/go.apexfission.aws.auth/application"
	"github.com/lambdawalker/go.apexfission.aws.auth/auth"
	"github.com/lambdawalker/go.apexfission.aws.auth/authorization"
	"github.com/lambdawalker/go.apexfission.aws.auth/awsemail"
	"github.com/lambdawalker/go.apexfission.aws.auth/awsidentity"
	"github.com/lambdawalker/go.apexfission.aws.auth/awsstore"
	"github.com/lambdawalker/go.apexfission.aws.auth/config"
	"github.com/lambdawalker/go.apexfission.aws.auth/email"
	sales "github.com/lambdawalker/go.apexfission.aws.auth/examples/sales"
	"github.com/lambdawalker/go.apexfission.aws.auth/passkey"
	"github.com/lambdawalker/go.apexfission.aws.auth/router"
	"github.com/lambdawalker/go.apexfission.aws.auth/signin"
	"log"
	"net/http"
	"os"
	"strings"
)

func required(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("missing %s", k)
	}
	return v
}
func main() {
	ctx := context.Background()
	cfg, e := awscfg.LoadDefaultConfig(ctx)
	if e != nil {
		log.Fatal("AWS configuration unavailable")
	}
	db := dynamodb.NewFromConfig(cfg)
	id := cognitoidentityprovider.NewFromConfig(cfg)
	var authModule auth.Module
	authModule.Lambda = "application"
	if os.Getenv("MODE") == "auth" {
		key, e := base64.StdEncoding.DecodeString(required("PROOF_KEY"))
		if e != nil || len(key) < 32 {
			log.Fatal("invalid proof key")
		}
		authModule.Email = &email.Service{Store: awsstore.Store{DB: db, Table: required("TABLE_NAME")}, Identity: awsidentity.Identity{Cognito: id, DB: db, Table: required("TABLE_NAME"), PoolID: required("POOL_ID"), ClientID: required("CLIENT_ID")}, Sender: awsemail.Sender{Client: sesv2.NewFromConfig(cfg), From: required("SENDER_ADDRESS")}, Key: key, Origin: required("APP_ORIGIN")}
		authModule.Passkeys = passkey.Service{Cognito: id, Endpoint: "https://cognito-idp." + cfg.Region + ".amazonaws.com/"}
		authModule.Signin = signin.Service{Client: id, ClientID: required("CLIENT_ID")}
	}
	app, e := application.Compose(authModule, sales.Module{Store: sales.Dynamo{DB: db, Table: os.Getenv("ORDERS_TABLE")}})
	if e != nil {
		log.Fatal(e)
	}
	issuer, client := required("ISSUER"), required("CLIENT_ID")
	keys, e := auth.NewJWKS(issuer)
	if e != nil {
		log.Fatal(e)
	}
	v := auth.Verifier{Issuer: issuer, ClientID: client, Keys: keys}
	env := required("ENVIRONMENT")
	var h http.Handler
	if os.Getenv("MODE") == "admin" {
		grants := authorization.DynamoGrants{DB: db, Table: required("CONFIG_TABLE")}
		shell, e := app.Admin(v, grants, []string{env}, config.Dynamo{DB: db, Table: required("CONFIG_TABLE")})
		if e != nil {
			log.Fatal(e)
		}
		if domain := os.Getenv("LOGIN_DOMAIN"); domain != "" {
			shell.Login = &admin.Login{Domain: domain, ClientID: client, RedirectURI: required("ADMIN_ORIGIN") + "/admin"}
		}
		h = shell
	} else {
		z := authorization.AuthorizerFunc(func(_ context.Context, p authorization.Principal, _, permission string) error {
			if p.Subject != "" && (permission == "sales:read" || permission == "sales:write") {
				return nil
			}
			return authorization.ErrDenied
		})
		h, e = app.Handler(v, z, env)
		if e != nil {
			log.Fatal(e)
		}
	}
	if addr := os.Getenv("LOCAL_LISTEN"); addr != "" && strings.HasPrefix(addr, "127.0.0.1:") {
		log.Fatal(http.ListenAndServe(addr, h))
	}
	lambda.Start(router.Lambda(h))
}
