package main

import (
	"github.com/lambdawalker/go.apexfission.aws.auth/application"
	"github.com/lambdawalker/go.apexfission.aws.auth/auth"
	sales "github.com/lambdawalker/go.apexfission.aws.auth/examples/sales"
	infra "github.com/lambdawalker/go.apexfission.aws.auth/pulumi"
	"github.com/lambdawalker/go.apexfission.aws.auth/router"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() { pulumi.Run(provision) }
func provision(ctx *pulumi.Context) error {
	cfg := config.New(ctx, "")
	return compose(ctx, cfg.Require("appOrigin"), cfg.Require("adminOrigin"), cfg.Require("senderDomain"), cfg.Require("senderAddress"), cfg.RequireSecret("proofKey"), "../dist")
}
func compose(ctx *pulumi.Context, origin, adminOrigin, domain, sender string, key pulumi.StringInput, archives string) error {
	a, e := infra.NewAuth(ctx, "auth", infra.AuthArgs{Origin: origin, SenderDomain: domain, SenderAddress: sender, ProofKey: key, ChallengeArchive: archives + "/challenge.zip"})
	if e != nil {
		return e
	}
	orders, e := dynamodb.NewTable(ctx, "sales-orders", &dynamodb.TableArgs{BillingMode: pulumi.String("PAY_PER_REQUEST"), HashKey: pulumi.String("owner"), RangeKey: pulumi.String("id"), Attributes: dynamodb.TableAttributeArray{&dynamodb.TableAttributeArgs{Name: pulumi.String("owner"), Type: pulumi.String("S")}, &dynamodb.TableAttributeArgs{Name: pulumi.String("id"), Type: pulumi.String("S")}}, ServerSideEncryption: &dynamodb.TableServerSideEncryptionArgs{Enabled: pulumi.Bool(true)}, PointInTimeRecovery: &dynamodb.TablePointInTimeRecoveryArgs{Enabled: pulumi.Bool(true)}}, pulumi.Protect(true))
	if e != nil {
		return e
	}
	env := pulumi.StringMap{}
	for k, v := range a.Environment {
		env[k] = v
	}
	env["MODE"] = pulumi.String("auth")
	env["ORDERS_TABLE"] = orders.Name
	env["ENVIRONMENT"] = pulumi.String(ctx.Stack())
	fn, e := infra.NewFunction(ctx, "application", infra.FunctionArgs{Archive: archives + "/application.zip", Environment: env, Policy: a.Policy})
	if e != nil {
		return e
	}
	salesPolicy := pulumi.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["dynamodb:Query","dynamodb:PutItem"],"Resource":%q}]}`, orders.Arn)
	// Keep business data permissions in a separate role/function, even with the same binary.
	business, e := infra.NewFunction(ctx, "sales", infra.FunctionArgs{Archive: archives + "/application.zip", Environment: pulumi.StringMap{"MODE": pulumi.String("sales"), "ISSUER": a.Issuer, "CLIENT_ID": a.Client.ID(), "ORDERS_TABLE": orders.Name, "ENVIRONMENT": pulumi.String(ctx.Stack())}, Policy: salesPolicy})
	if e != nil {
		return e
	}
	app, e := application.Compose(auth.Module{Lambda: "application"}, sales.Module{})
	if e != nil {
		return e
	}
	routes := []router.Route{}
	for _, r := range app.Routes {
		if !r.Admin {
			if r.Permission != "" {
				r.Lambda = "sales"
			}
			routes = append(routes, r)
		}
	}
	api, e := infra.NewAPI(ctx, "application", infra.APIArgs{RuntimeEnforced: map[string]bool{"application": true, "sales": true}, Routes: routes, Functions: map[string]*lambda.Function{"application": fn, "sales": business}, Issuer: a.Issuer, ClientID: a.Client.ID(), Origin: origin})
	if e != nil {
		return e
	}
	admin, e := infra.NewAdminWithLogin(ctx, "admin", adminOrigin)
	if e != nil {
		return e
	}
	adminFn, e := infra.NewFunction(ctx, "admin", infra.FunctionArgs{Archive: archives + "/application.zip", Environment: pulumi.StringMap{"MODE": pulumi.String("admin"), "LOGIN_DOMAIN": admin.LoginDomain, "ADMIN_ORIGIN": pulumi.String(adminOrigin), "ISSUER": admin.Issuer, "CLIENT_ID": admin.Client.ID(), "CONFIG_TABLE": admin.Table.Name, "ENVIRONMENT": pulumi.String(ctx.Stack())}, Policy: pulumi.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["dynamodb:GetItem","dynamodb:Query","dynamodb:PutItem","dynamodb:ConditionCheckItem"],"Resource":%q,"Condition":{"ForAllValues:StringLike":{"dynamodb:LeadingKeys":["ENV#%s#MODULE#*"]}}},{"Effect":"Allow","Action":["dynamodb:GetItem","dynamodb:ConditionCheckItem"],"Resource":%q,"Condition":{"ForAllValues:StringEquals":{"dynamodb:LeadingKeys":["GRANT#%s"]}}}]}`, admin.Table.Arn, ctx.Stack(), admin.Table.Arn, ctx.Stack())})
	if e != nil {
		return e
	}
	adminAPI, e := infra.NewAdminAPI(ctx, "admin", adminFn, true)
	if e != nil {
		return e
	}
	ctx.Export("apiUrl", api.URL)
	ctx.Export("adminUrl", pulumi.Sprintf("%s/admin", adminAPI.URL))
	ctx.Export("userPoolId", a.Pool.ID())
	ctx.Export("clientId", a.Client.ID())
	ctx.Export("adminPoolId", admin.Pool.ID())
	ctx.Export("adminClientId", admin.Client.ID())
	ctx.Export("configTable", admin.Table.Name)
	ctx.Export("ordersTable", orders.Name)
	ctx.Export("sesVerification", a.Sender.VerificationToken)
	ctx.Export("dkimTokens", a.DKIM.DkimTokens)
	return nil
}
