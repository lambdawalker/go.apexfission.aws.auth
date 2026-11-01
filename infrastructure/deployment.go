// Package infrastructure deploys the native authentication backend. One backend
// per Pulumi stack is supported; internal resource names are intentionally stable.
package infrastructure

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/lambdawalker/go.apexfission.aws.auth/artifacts"
	"github.com/lambdawalker/go.apexfission.aws.auth/assetlinks"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/apigatewayv2"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Config accepts Pulumi config.Config or a caller-supplied supported reader.
// RequireSecret must return a secret-marked Pulumi output.
type Config interface {
	Get(string) string
	Require(string) string
	RequireSecret(string) pulumi.StringOutput
	GetObject(string, interface{}) error
	GetBool(string) bool
}

type Options struct {
	ArtifactDir       string
	RuntimeTableName  pulumi.StringInput
	RuntimeTableARN   pulumi.StringInput
	Environment       string
	ResetFunctionName string
}

type Deployment struct {
	RuntimeTableName pulumi.StringOutput
	RuntimeTableARN  pulumi.StringOutput
	// RuntimeTable is non-nil only when DeployStandalone creates the isolated table.
	RuntimeTable      *dynamodb.Table
	APIURL            pulumi.StringOutput
	PoolID            pulumi.StringOutput
	ClientID          pulumi.StringOutput
	Issuer            pulumi.StringOutput
	ResetFunctionName pulumi.StringOutput
	API               *apigatewayv2.Api
	Stage             *apigatewayv2.Stage
	ResetFunction     *lambda.Function
}

func requiredArchiveNames() []string {
	entries := artifacts.Lambdas()
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name
	}
	return names
}

// Validate checks settings and every required archive without registering resources.
// Runtime table values remain Pulumi inputs; their values need not resolve here.
func Validate(cfg Config, opts Options) error {
	if opts.RuntimeTableName == nil || opts.RuntimeTableARN == nil {
		return fmt.Errorf("RuntimeTableName and RuntimeTableARN bindings are required; supply the shared runtime table outputs through infrastructure.Options")
	}
	for key, value := range map[string]pulumi.StringInput{"RuntimeTableName": opts.RuntimeTableName, "RuntimeTableARN": opts.RuntimeTableARN} {
		if v, ok := value.(pulumi.String); ok && strings.TrimSpace(string(v)) == "" {
			return fmt.Errorf("%s binding must not be empty", key)
		}
	}
	if cfg == nil {
		return fmt.Errorf("configuration reader is required")
	}
	if opts.Environment == "" {
		return fmt.Errorf("Environment is required for standalone preflight")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9-_]{1,64}$`).MatchString(opts.ResetFunctionName) {
		return fmt.Errorf("ResetFunctionName must be a distinct application-owned Lambda function name (1-64 letters, digits, hyphens or underscores)")
	}
	if err := validate(cfg.Get("appOrigin"), cfg.Get("senderDomain"), cfg.Get("senderAddress")); err != nil {
		return err
	}
	key := cfg.Get("proofKey")
	if key == "" {
		return fmt.Errorf("proofKey must be supplied as Pulumi secret configuration")
	}
	b, e := base64.StdEncoding.DecodeString(key)
	if e != nil || len(b) < 32 {
		return fmt.Errorf("proofKey must be base64 of at least 32 random bytes")
	}
	if domain := cfg.Get("apiDomain"); domain != "" {
		if len(domain) > 253 || !strings.Contains(domain, ".") {
			return fmt.Errorf("apiDomain must be a DNS hostname")
		}
		for _, label := range strings.Split(domain, ".") {
			if len(label) > 63 || !regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`).MatchString(label) {
				return fmt.Errorf("apiDomain must be a lowercase DNS hostname")
			}
		}
	}
	if cfg.Get("androidAssociations") != "" {
		var apps []assetlinks.Application
		if err := cfg.GetObject("androidAssociations", &apps); err != nil {
			return err
		}
		if _, err := assetlinks.Render(apps); err != nil {
			return err
		}
		if strings.Contains(strings.TrimPrefix(cfg.Get("appOrigin"), "https://"), ":") {
			return fmt.Errorf("asset links require an HTTPS appOrigin hostname")
		}
	}
	if _, err := captureCapacity(cfg, opts.Environment); err != nil {
		return err
	}
	for _, name := range []string{"environmentResetEnabled", "captureEnabled", "diagnosticMode"} {
		if v := cfg.Get(name); v != "" {
			if _, err := strconv.ParseBool(v); err != nil {
				return fmt.Errorf("%s must be boolean", name)
			}
		}
	}
	defaults := deploymentRuntimeDefaults(opts.Environment, cfg.Get("captureEnabled"))
	if err := defaults.Validate(); err != nil {
		return err
	}
	if opts.ArtifactDir == "" {
		return fmt.Errorf("ArtifactDir is required")
	}
	for _, name := range requiredArchiveNames() {
		path := filepath.Join(opts.ArtifactDir, name+".zip")
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("build Lambda archive %s before deployment: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Lambda archive %s must be a regular file", path)
		}
	}
	return nil
}
