package infrastructure

import "github.com/lambdawalker/go.infra.composer/adapters/pulumi/awslabels"

import (
	"crypto/sha256"
	"fmt"
	"regexp"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/dynamodb"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// DeployStandalone provisions an isolated runtime table when neither runtime
// binding is provided. Both supplied bindings are reused without creating a table.
// All backend settings and archives are checked before creating the table.
// Runtime policy records must be seeded separately after deployment.
func DeployStandalone(ctx *pulumi.Context, cfg Config, opts Options) (*Deployment, error) {
	if opts.Environment == "" {
		opts.Environment = ctx.Stack()
	}
	if opts.ResetFunctionName == "" {
		name := "apexfission-auth-reset-" + ctx.Project() + "-" + ctx.Stack()
		name = regexp.MustCompile(`[^A-Za-z0-9_-]`).ReplaceAllString(name, "-")
		if len(name) > 64 {
			name = fmt.Sprintf("%s-%x", name[:47], sha256.Sum256([]byte(name)))[:64]
		}
		opts.ResetFunctionName = name
	}
	missingName := opts.RuntimeTableName == nil
	missingARN := opts.RuntimeTableARN == nil
	if missingName != missingARN {
		return nil, fmt.Errorf("RuntimeTableName and RuntimeTableARN must both be provided or both omitted for standalone deployment")
	}
	if !missingName {
		return Deploy(ctx, cfg, opts)
	}
	checked := opts
	checked.RuntimeTableName = pulumi.String("standalone-preflight-table")
	checked.RuntimeTableARN = pulumi.String("arn:standalone-preflight-table")
	if err := Validate(cfg, checked); err != nil {
		return nil, err
	}
	ctx, labels, err := awslabels.Ensure(ctx, opts.Environment, cfg.Get("appOrigin"), "authentication")
	if err != nil {
		return nil, err
	}
	table, err := dynamodb.NewTable(ctx, "auth-runtime-configuration", &dynamodb.TableArgs{
		BillingMode: pulumi.String("PAY_PER_REQUEST"), HashKey: pulumi.String("id"), RangeKey: pulumi.String("sk"),
		Attributes:           dynamodb.TableAttributeArray{&dynamodb.TableAttributeArgs{Name: pulumi.String("id"), Type: pulumi.String("S")}, &dynamodb.TableAttributeArgs{Name: pulumi.String("sk"), Type: pulumi.String("S")}},
		ServerSideEncryption: &dynamodb.TableServerSideEncryptionArgs{Enabled: pulumi.Bool(true)}, PointInTimeRecovery: &dynamodb.TablePointInTimeRecoveryArgs{Enabled: pulumi.Bool(true)}, DeletionProtectionEnabled: pulumi.Bool(true),
	}, awslabels.Option(ctx))
	if err != nil {
		return nil, err
	}
	opts.RuntimeTableName = table.Name
	opts.RuntimeTableARN = table.Arn
	d, err := Deploy(ctx, cfg, opts)
	if err != nil {
		return nil, err
	}
	d.RuntimeTable = table
	if labels != nil {
		if err := labels.Export(ctx, "resourceInventory"); err != nil {
			return nil, err
		}
	}
	return d, nil
}
