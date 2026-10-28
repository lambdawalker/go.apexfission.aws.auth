package infrastructure

import (
	"fmt"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/apigatewayv2"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// NewAdminAPI forwards every path to an admin.Shell, which validates JWTs from
// bearer headers or HttpOnly cookies and authorizes every API and frontend asset.
// There is deliberately no gateway JWT authorizer: it cannot read this session cookie.
func NewAdminAPI(ctx *pulumi.Context, name string, fn *lambda.Function, runtimeEnforced bool, opts ...pulumi.ResourceOption) (*API, error) {
	if fn == nil || !runtimeEnforced {
		return nil, fmt.Errorf("admin integration must use a runtime-authenticated admin shell")
	}
	a := &API{}
	if e := ctx.RegisterComponentResource("apexfission:admin:HTTP", name, a, opts...); e != nil {
		return nil, e
	}
	var e error
	a.API, e = apigatewayv2.NewApi(ctx, name+"-http", &apigatewayv2.ApiArgs{ProtocolType: pulumi.String("HTTP")}, pulumi.Parent(a))
	if e != nil {
		return nil, e
	}
	i, e := apigatewayv2.NewIntegration(ctx, name+"-integration", &apigatewayv2.IntegrationArgs{ApiId: a.API.ID(), IntegrationType: pulumi.String("AWS_PROXY"), IntegrationUri: fn.InvokeArn, PayloadFormatVersion: pulumi.String("2.0")}, pulumi.Parent(a))
	if e != nil {
		return nil, e
	}
	if _, e = apigatewayv2.NewRoute(ctx, name+"-route", &apigatewayv2.RouteArgs{ApiId: a.API.ID(), RouteKey: pulumi.String("$default"), Target: pulumi.Sprintf("integrations/%s", i.ID())}, pulumi.Parent(a)); e != nil {
		return nil, e
	}
	if _, e = lambda.NewPermission(ctx, name+"-invoke", &lambda.PermissionArgs{Action: pulumi.String("lambda:InvokeFunction"), Function: fn.Name, Principal: pulumi.String("apigateway.amazonaws.com"), SourceArn: pulumi.Sprintf("%s/*", a.API.ExecutionArn)}, pulumi.Parent(a)); e != nil {
		return nil, e
	}
	if _, e = apigatewayv2.NewStage(ctx, name+"-stage", &apigatewayv2.StageArgs{ApiId: a.API.ID(), Name: pulumi.String("$default"), AutoDeploy: pulumi.Bool(true), DefaultRouteSettings: &apigatewayv2.StageDefaultRouteSettingsArgs{ThrottlingBurstLimit: pulumi.Int(20), ThrottlingRateLimit: pulumi.Float64(10)}}, pulumi.Parent(a)); e != nil {
		return nil, e
	}
	a.URL = a.API.ApiEndpoint
	return a, ctx.RegisterResourceOutputs(a, pulumi.Map{"url": a.URL})
}
