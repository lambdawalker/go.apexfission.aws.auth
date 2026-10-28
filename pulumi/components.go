// Package infrastructure provides application-owned Pulumi resources.
package infrastructure

import (
	"crypto/sha256"
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/router"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/apigatewayv2"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"regexp"
	"strings"
)

func NewTable(ctx *pulumi.Context, name, key string, opts ...pulumi.ResourceOption) (*dynamodb.Table, error) {
	return dynamodb.NewTable(ctx, name, &dynamodb.TableArgs{BillingMode: pulumi.String("PAY_PER_REQUEST"), HashKey: pulumi.String(key), Attributes: dynamodb.TableAttributeArray{&dynamodb.TableAttributeArgs{Name: pulumi.String(key), Type: pulumi.String("S")}}, ServerSideEncryption: &dynamodb.TableServerSideEncryptionArgs{Enabled: pulumi.Bool(true)}, PointInTimeRecovery: &dynamodb.TablePointInTimeRecoveryArgs{Enabled: pulumi.Bool(true)}}, append([]pulumi.ResourceOption{pulumi.Protect(true)}, opts...)...)
}

type FunctionArgs struct {
	Archive     string
	Environment pulumi.StringMap
	Policy      pulumi.StringInput
	Timeout     int
}

func NewFunction(ctx *pulumi.Context, name string, a FunctionArgs, opts ...pulumi.ResourceOption) (*lambda.Function, error) {
	if a.Archive == "" {
		return nil, fmt.Errorf("%s requires a built archive", name)
	}
	role, e := iam.NewRole(ctx, name+"-role", &iam.RoleArgs{AssumeRolePolicy: pulumi.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}`)}, opts...)
	if e != nil {
		return nil, e
	}
	logs, e := cloudwatch.NewLogGroup(ctx, name+"-logs", &cloudwatch.LogGroupArgs{RetentionInDays: pulumi.Int(30)}, opts...)
	if e != nil {
		return nil, e
	}
	logging, e := iam.NewRolePolicy(ctx, name+"-logging", &iam.RolePolicyArgs{Role: role.Name, Policy: pulumi.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["logs:CreateLogStream","logs:PutLogEvents"],"Resource":"%s:*"}]}`, logs.Arn)}, opts...)
	if e != nil {
		return nil, e
	}
	deps := []pulumi.Resource{logging}
	if a.Policy != nil {
		p, e := iam.NewRolePolicy(ctx, name+"-permissions", &iam.RolePolicyArgs{Role: role.Name, Policy: a.Policy}, opts...)
		if e != nil {
			return nil, e
		}
		deps = append(deps, p)
	}
	timeout := a.Timeout
	if timeout == 0 {
		timeout = 25
	}
	return lambda.NewFunction(ctx, name, &lambda.FunctionArgs{Runtime: pulumi.String("provided.al2023"), Handler: pulumi.String("bootstrap"), Architectures: pulumi.StringArray{pulumi.String("arm64")}, Code: pulumi.NewFileArchive(a.Archive), Role: role.Arn, Timeout: pulumi.Int(timeout), MemorySize: pulumi.Int(256), Environment: &lambda.FunctionEnvironmentArgs{Variables: a.Environment}, LoggingConfig: &lambda.FunctionLoggingConfigArgs{LogFormat: pulumi.String("Text"), LogGroup: logs.Name}}, append(opts, pulumi.DependsOn(deps))...)
}

type APIArgs struct {
	// RuntimeEnforced explicitly attests that each protected Lambda verifies access-token use and enforces route permissions/validation.
	RuntimeEnforced  map[string]bool
	Routes           []router.Route
	Functions        map[string]*lambda.Function
	Issuer, ClientID pulumi.StringInput
	Origin           string
}
type API struct {
	pulumi.ResourceState
	API *apigatewayv2.Api
	URL pulumi.StringOutput
}

var parameter = regexp.MustCompile(`\{[^}]+\}`)

func NewAPI(ctx *pulumi.Context, name string, a APIArgs, opts ...pulumi.ResourceOption) (*API, error) {
	for i, r := range a.Routes {
		if e := router.Validate(r); e != nil {
			return nil, e
		}
		if a.Functions[r.Lambda] == nil {
			return nil, fmt.Errorf("route %s missing Lambda %s", r.Path, r.Lambda)
		}
		if r.Access != router.Public && !a.RuntimeEnforced[r.Lambda] {
			return nil, fmt.Errorf("protected Lambda %s must explicitly enforce runtime token validation and policies", r.Lambda)
		}
		if r.Access != router.Public && (a.Issuer == nil || a.ClientID == nil) {
			return nil, fmt.Errorf("protected route requires issuer and client")
		}
		for _, old := range a.Routes[:i] {
			if router.Conflicts(old, r) {
				return nil, fmt.Errorf("conflicting API route")
			}
		}
	}
	c := &API{}
	if e := ctx.RegisterComponentResource("apexfission:api:HTTP", name, c, opts...); e != nil {
		return nil, e
	}
	child := []pulumi.ResourceOption{pulumi.Parent(c)}
	args := &apigatewayv2.ApiArgs{ProtocolType: pulumi.String("HTTP")}
	if a.Origin != "" {
		args.CorsConfiguration = &apigatewayv2.ApiCorsConfigurationArgs{AllowOrigins: pulumi.StringArray{pulumi.String(a.Origin)}, AllowMethods: pulumi.StringArray{pulumi.String("GET"), pulumi.String("POST"), pulumi.String("PUT"), pulumi.String("DELETE"), pulumi.String("PATCH")}, AllowHeaders: pulumi.StringArray{pulumi.String("authorization"), pulumi.String("content-type")}}
	}
	var e error
	c.API, e = apigatewayv2.NewApi(ctx, name+"-http", args, child...)
	if e != nil {
		return nil, e
	}
	var authorizer *apigatewayv2.Authorizer
	if a.Issuer != nil && a.ClientID != nil {
		authorizer, e = apigatewayv2.NewAuthorizer(ctx, name+"-jwt", &apigatewayv2.AuthorizerArgs{ApiId: c.API.ID(), AuthorizerType: pulumi.String("JWT"), IdentitySources: pulumi.StringArray{pulumi.String("$request.header.Authorization")}, JwtConfiguration: &apigatewayv2.AuthorizerJwtConfigurationArgs{Issuer: a.Issuer, Audiences: pulumi.StringArray{a.ClientID}}}, child...)
		if e != nil {
			return nil, e
		}
	}
	for _, r := range a.Routes {
		hash := sha256.Sum256([]byte(r.Method + " " + r.Path))
		id := fmt.Sprintf("%s-%x", name, hash[:6])
		fn := a.Functions[r.Lambda]
		integration, e := apigatewayv2.NewIntegration(ctx, id+"-integration", &apigatewayv2.IntegrationArgs{ApiId: c.API.ID(), IntegrationType: pulumi.String("AWS_PROXY"), IntegrationUri: fn.InvokeArn, PayloadFormatVersion: pulumi.String("2.0")}, child...)
		if e != nil {
			return nil, e
		}
		args := &apigatewayv2.RouteArgs{ApiId: c.API.ID(), RouteKey: pulumi.String(r.Method + " " + r.Path), Target: pulumi.Sprintf("integrations/%s", integration.ID()), AuthorizationType: pulumi.String("NONE")}
		if r.Access != router.Public {
			args.AuthorizationType = pulumi.String("JWT")
			args.AuthorizerId = authorizer.ID()
		}
		if _, e = apigatewayv2.NewRoute(ctx, id+"-route", args, child...); e != nil {
			return nil, e
		}
		routePath := parameter.ReplaceAllString(strings.TrimPrefix(r.Path, "/"), "*")
		if _, e = lambda.NewPermission(ctx, id+"-invoke", &lambda.PermissionArgs{Action: pulumi.String("lambda:InvokeFunction"), Function: fn.Name, Principal: pulumi.String("apigateway.amazonaws.com"), SourceArn: pulumi.Sprintf("%s/*/%s/%s", c.API.ExecutionArn, r.Method, routePath)}, child...); e != nil {
			return nil, e
		}
	}
	if _, e = apigatewayv2.NewStage(ctx, name+"-stage", &apigatewayv2.StageArgs{ApiId: c.API.ID(), Name: pulumi.String("$default"), AutoDeploy: pulumi.Bool(true), DefaultRouteSettings: &apigatewayv2.StageDefaultRouteSettingsArgs{ThrottlingBurstLimit: pulumi.Int(20), ThrottlingRateLimit: pulumi.Float64(10)}}, child...); e != nil {
		return nil, e
	}
	c.URL = c.API.ApiEndpoint
	if e = ctx.RegisterResourceOutputs(c, pulumi.Map{"url": c.URL}); e != nil {
		return nil, e
	}
	return c, nil
}

// Extension runs inside the application's stack; it can return resources and outputs.
type Extension interface {
	Provision(*pulumi.Context) (pulumi.Map, error)
}
