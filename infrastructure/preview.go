package infrastructure

import (
	"github.com/lambdawalker/go.apexfission.aws.auth/artifacts"
	"github.com/lambdawalker/go.infra.composer/adapters/pulumi/reportpreview"
	"github.com/lambdawalker/go.infra.composer/reporting"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Preview constructs the actual native feature offline. External host bindings
// and private empty archives are fixtures; no provider or deployment is invoked.
func Preview(config reportpreview.Config, environment string) (out []reporting.Artifact, err error) {
	cfg := reportpreview.Defaults(config)
	names := []string{}
	for _, a := range artifacts.Lambdas() {
		names = append(names, a.Name)
	}
	err = reportpreview.Archives(names, func(dir string) error {
		var e error
		out, e = reportpreview.Catalog(environment, "authentication", "go.apexfission.aws.auth", func(ctx *pulumi.Context) error {
			opts := Options{ArtifactDir: dir, Environment: environment}
			if cfg.Get("suppliedRuntime") == "true" {
				opts.RuntimeTableName = pulumi.String("external-runtime")
				opts.RuntimeTableARN = pulumi.String("arn:aws:dynamodb:us-east-1:000000000000:table/external-runtime")
			}
			_, err := DeployStandalone(ctx, cfg, opts)
			return err
		})
		return e
	})
	return
}
