// Standalone deployment uses the same native backend as composed applications.
package main

import (
	"github.com/lambdawalker/go.apexfission.aws.auth/infrastructure"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		opts := infrastructure.Options{ArtifactDir: cfg.Get("artifactDir"), Environment: ctx.Stack(), ResetFunctionName: cfg.Get("resetFunctionName")}
		if name := cfg.Get("runtimeTableName"); name != "" {
			opts.RuntimeTableName = pulumi.String(name)
		}
		if arn := cfg.Get("runtimeTableArn"); arn != "" {
			opts.RuntimeTableARN = pulumi.String(arn)
		}
		d, err := infrastructure.DeployStandalone(ctx, cfg, opts)
		if err != nil {
			return err
		}
		ctx.Export("runtimeTableName", d.RuntimeTableName)
		ctx.Export("runtimeTableArn", d.RuntimeTableARN)
		ctx.Export("environment", pulumi.String(ctx.Stack()))
		ctx.Export("apiUrl", d.APIURL)
		ctx.Export("userPoolId", d.PoolID)
		ctx.Export("clientId", d.ClientID)
		ctx.Export("issuer", d.Issuer)
		ctx.Export("resetFunctionName", d.ResetFunctionName)
		return nil
	})
}
