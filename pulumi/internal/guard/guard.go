// Package guard validates deployment identity before resources are registered.
package guard

import (
	"fmt"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func Check(ctx *pulumi.Context, scope string) error {
	cfg := config.New(ctx, "")
	prefix, err := cfg.Try("deploymentPrefix")
	if err != nil {
		return fmt.Errorf("deploymentPrefix must be explicitly configured (empty selects final): %w", err)
	}
	identity := prefix
	if identity == "" {
		identity = "final"
	}
	if !regexp.MustCompile(`^[a-z][a-z0-9]{0,19}$`).MatchString(identity) || prefix == "final" {
		return fmt.Errorf("invalid or reserved deploymentPrefix")
	}
	suffix := cfg.Get("environment")
	if scope == "shared" {
		suffix = "shared"
	} else if !regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`).MatchString(suffix) || suffix == "shared" {
		return fmt.Errorf("environment must be a safe logical backend environment")
	}
	if len(identity+"-"+suffix) > 32 {
		return fmt.Errorf("full deployment stack exceeds registry environment limit of 32 characters")
	}
	if ctx.Project() != "attestra-"+scope || ctx.Stack() != identity+"-"+suffix {
		return fmt.Errorf("refusing legacy or mismatched project/stack: expected attestra-%s/%s-%s", scope, identity, suffix)
	}
	account := cfg.Get("expectedAccountId")
	region := cfg.Get("expectedRegion")
	if !regexp.MustCompile(`^[0-9]{12}$`).MatchString(account) || region == "" {
		return fmt.Errorf("expectedAccountId and expectedRegion pins are required")
	}
	caller, err := aws.GetCallerIdentity(ctx, nil)
	if err != nil {
		return err
	}
	r, err := aws.GetRegion(ctx, nil)
	if err != nil {
		return err
	}
	if caller.AccountId != account || r.Name != region {
		return fmt.Errorf("AWS account/region does not match deployment pins")
	}
	if scope == "backend" {
		if cfg.Get("sharedStack") != identity+"-shared" || cfg.Get("sharedAccountId") != account || cfg.Get("sharedRegion") != region {
			return fmt.Errorf("shared output identity does not match backend deployment")
		}
		if cfg.Get("runtimeTableName") == "" || cfg.Get("runtimeTableArn") != "arn:aws:dynamodb:"+region+":"+account+":table/"+cfg.Get("runtimeTableName") {
			return fmt.Errorf("runtimeTableArn must match runtimeTableName in pinned account and region")
		}
	}
	for _, key := range []string{"apiCertificateArn", "indexCertificateArn", "adminCertificateArn", "appCertificateArn"} {
		if arn := cfg.Get(key); arn != "" {
			certRegion := region
			if key == "appCertificateArn" {
				certRegion = "us-east-1"
			}
			if !strings.HasPrefix(arn, "arn:aws:acm:"+certRegion+":"+account+":certificate/") {
				return fmt.Errorf("%s must reference pinned account and expected certificate region", key)
			}
		}
	}
	required := []string{"domain", "adminDomain", "sourceHash"}
	if scope == "backend" {
		required = []string{"runtimeTableName", "runtimeTableArn", "appOrigin", "senderDomain", "senderAddress", "senderIdentityArn", "proofKey"}
	}
	for _, key := range required {
		if cfg.Get(key) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	ctx.Export("deploymentPrefix", pulumi.String(prefix))
	ctx.Export("deploymentIdentity", pulumi.String(identity))
	ctx.Export("deploymentAccountId", pulumi.String(account))
	ctx.Export("deploymentRegion", pulumi.String(region))
	ctx.Export("deploymentStack", pulumi.String(ctx.Stack()))
	return nil
}

// Archives rejects incomplete build output before resource registration.
func Archives(names ...string) error {
	for _, name := range names {
		path := filepath.Join("..", "dist", name+".zip")
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("build Lambda archive %s first: %w", path, err)
		}
	}
	return nil
}
