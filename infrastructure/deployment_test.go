package infrastructure

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func testConfig(t *testing.T) {
	t.Helper()
	t.Setenv("PULUMI_CONFIG", `{"test:appOrigin":"https://dev.example.com","test:senderDomain":"example.com","test:senderAddress":"hello@example.com","test:proofKey":"`+base64.StdEncoding.EncodeToString(make([]byte, 32))+`"}`)
}
func TestMissingLastArchiveRegistersNothing(t *testing.T) {
	testConfig(t)
	dir := t.TempDir()
	for _, name := range requiredArchiveNames()[:len(requiredArchiveNames())-1] {
		if err := os.WriteFile(filepath.Join(dir, name+".zip"), []byte("archive"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m := &associationMocks{resources: map[string]pulumi.MockResourceArgs{}}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, err := Deploy(ctx, config.New(ctx, "test"), Options{ArtifactDir: dir, RuntimeTableName: pulumi.String("runtime"), RuntimeTableARN: pulumi.String("arn:runtime"), Environment: "test", ResetFunctionName: "application-reset-test"})
		return err
	}, pulumi.WithMocks("test", "test", m))
	if err == nil || !strings.Contains(err.Error(), "environment-reset.zip") {
		t.Fatalf("missing archive error: %v", err)
	}
	for _, r := range m.resources {
		if strings.HasPrefix(r.TypeToken, "aws:") {
			t.Fatalf("registered %s before validation", r.TypeToken)
		}
	}
}

type backendMocks struct{ associationMocks }

func (m *backendMocks) Call(a pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.NewPropertyMapFromMap(map[string]interface{}{"name": "us-east-2", "accountId": "123456789012"}), nil
}
func (m *backendMocks) NewResource(a pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	id, out, err := m.associationMocks.NewResource(a)
	for key, value := range map[string]string{"name": a.Name, "versionId": "version-1", "apiEndpoint": "https://api.example.com", "executionArn": "arn:api", "url": "https://sqs.example.com/queue", "invokeArn": "arn:invoke"} {
		if !out[resource.PropertyKey(key)].HasValue() {
			out[resource.PropertyKey(key)] = resource.NewStringProperty(value)
		}
	}
	return id, out, err
}
func TestNativeDeploymentRetainsRuntimeAndSecretBindings(t *testing.T) {
	testConfig(t)
	dir := t.TempDir()
	for _, name := range requiredArchiveNames() {
		if err := os.WriteFile(filepath.Join(dir, name+".zip"), []byte("archive"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m := &backendMocks{associationMocks{resources: map[string]pulumi.MockResourceArgs{}}}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		// Outputs from a registered native dependency remain outputs all the way to AWS inputs.
		var shared pulumi.CustomResourceState
		if err := ctx.RegisterResource("test:index:Runtime", "shared", pulumi.Map{}, &shared); err != nil {
			return err
		}
		name := shared.ID().ApplyT(func(id pulumi.ID) string { return "shared-runtime-" + string(id) }).(pulumi.StringOutput)
		arn := shared.ID().ApplyT(func(id pulumi.ID) string { return "arn:runtime:" + string(id) }).(pulumi.StringOutput)
		d, err := Deploy(ctx, config.New(ctx, "test"), Options{ArtifactDir: dir, RuntimeTableName: name, RuntimeTableARN: arn, Environment: "isolated", ResetFunctionName: "application-reset-isolated"})
		if err != nil {
			return err
		}
		if d.API == nil || d.Stage == nil {
			return os.ErrInvalid
		}
		return nil
	}, pulumi.WithMocks("test", "test", m))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range m.resources {
		if r.Name == "email-signup" || r.Name == "email-proof-key" || r.Name == "verification-proofs" {
			t.Fatalf("verification ownership leaked: %s", r.Name)
		}
	}
	if got := m.resources["cognito-grant-challenge"].Inputs["description"].StringValue(); got != "dev.example.com ISOLATED | go.apexfission.aws.auth" {
		t.Fatal(got)
	}
	count := 0
	for _, r := range m.resources {
		if r.TypeToken != "aws:lambda/function:Function" {
			continue
		}
		count++
		env := r.Inputs["environment"]
		if !env.HasValue() {
			continue
		}
		vars := env.ObjectValue()["variables"].ObjectValue()
		if _, ok := vars["PROOF_KEY"]; ok {
			t.Fatalf("raw proof key in %s", r.Name)
		}
		if r.Name == "email-signup" || r.Name == "email-resend" || r.Name == "email-confirm" {
			if vars["PROOF_KEY_SECRET_ARN"].StringValue() != "arn:example:email-proof-key" || vars["PROOF_KEY_SECRET_VERSION"].StringValue() != "version-1" {
				t.Fatalf("missing secret reference for %s", r.Name)
			}
		} else {
			if _, ok := vars["PROOF_KEY_SECRET_ARN"]; ok {
				t.Fatalf("unexpected secret consumer %s", r.Name)
			}
		}
		if r.Name == "app-config" || r.Name == "auth-status" || r.Name == "auth-password-set" {
			if vars["RUNTIME_TABLE"].StringValue() != "shared-runtime-shared-id" || vars["ENVIRONMENT"].StringValue() != "isolated" {
				t.Fatalf("runtime binding lost for %s: %v", r.Name, vars)
			}
		}
	}
	if count != 14 {
		t.Fatalf("got %d Lambda functions; want 14", count)
	}
	for _, name := range []string{"app-config-data", "auth-status-enrollment", "auth-password-set-enrollment"} {
		if !strings.Contains(m.resources[name].Inputs["policy"].StringValue(), "arn:runtime:shared-id") {
			t.Fatalf("deferred ARN lost in %s", name)
		}
	}

	for _, name := range []string{"app-config", "auth-status", "auth-password-set"} {
		rpc := m.resources[name].RegisterRPC
		if rpc == nil {
			t.Fatalf("missing native registration for %s", name)
		}
		dep := rpc.GetPropertyDependencies()["environment"]
		found := false
		for _, urn := range dep.GetUrns() {
			if strings.HasSuffix(urn, "::shared") {
				found = true
			}
		}
		if !found {
			t.Fatalf("shared runtime native dependency lost for %s: %v", name, dep)
		}
	}
	if m.resources["environment-reset"].Inputs["name"].StringValue() != "application-reset-isolated" {
		t.Fatal("reset physical name lost")
	}
}

func TestCaptureFactsInjectionDoesNotTransferOwnership(t *testing.T) {
	for _, injected := range []bool{false, true} {
		t.Run(map[bool]string{false: "standalone", true: "injected"}[injected], func(t *testing.T) {
			testConfig(t)
			dir := t.TempDir()
			for _, name := range requiredArchiveNames() {
				if err := os.WriteFile(filepath.Join(dir, name+".zip"), []byte("archive"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			m := &backendMocks{associationMocks{resources: map[string]pulumi.MockResourceArgs{}}}
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				opts := Options{ArtifactDir: dir, RuntimeTableName: pulumi.String("runtime"), RuntimeTableARN: pulumi.String("arn:runtime"), Environment: "test", ResetFunctionName: "auth-reset-test"}
				if injected {
					opts.CaptureTableName = pulumi.String("external-captures")
					opts.CaptureTableARN = pulumi.String("arn:external-captures")
				}
				d, err := Deploy(ctx, config.New(ctx, "test"), opts)
				if err != nil {
					return err
				}
				if d.ResetInventory.CaptureTable != nil || d.ResetInventory.CaptureTableARN != nil || len(d.ResetInventory.Tables) != 1 || len(d.ResetInventory.Functions) != 12 {
					t.Fatal("capture resources leaked into auth reset inventory", d.ResetInventory)
				}
				if len(d.CaptureIntegration) != 8 {
					t.Fatal("incomplete capture integration", d.CaptureIntegration)
				}
				defaults := deploymentRuntimeDefaults(opts.Environment, captureSetting(config.New(ctx, "test"), opts))
				if defaults.Features.IDCapture != injected || (defaults.Onboarding.IDCapture == "disabled") == injected {
					t.Fatal(defaults)
				}
				return nil
			}, pulumi.WithMocks("test", "test", m))
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, r := range m.resources {
				if strings.HasPrefix(r.Name, "capture") || r.Name == "id-captures" || strings.HasPrefix(r.TypeToken, "aws:sqs/") || strings.HasPrefix(r.TypeToken, "aws:s3/") {
					t.Fatalf("capture resource registered by auth: %s %s", r.Name, r.TypeToken)
				}
				if r.TypeToken == "aws:lambda/function:Function" {
					count++
				}
			}
			if count != 14 {
				t.Fatal("incorrect authentication Lambda count", count)
			}
			vars := m.resources["auth-status"].Inputs["environment"].ObjectValue()["variables"].ObjectValue()
			if injected {
				if vars["CAPTURE_TABLE"].StringValue() != "external-captures" {
					t.Fatal("injected table not bound", vars)
				}
			} else if _, ok := vars["CAPTURE_TABLE"]; ok {
				t.Fatal("captureless status received capture table", vars)
			}
			resetPolicy := m.resources["environment-reset-data"].Inputs["policy"].StringValue()
			if strings.Contains(resetPolicy, "s3:") || strings.Contains(resetPolicy, "sqs:") || strings.Contains(resetPolicy, "arn:external-captures") {
				t.Fatal("auth reset claims capture resources", resetPolicy)
			}
			policy := m.resources["auth-status-enrollment"].Inputs["policy"].StringValue()
			if strings.Contains(policy, "arn:external-captures") != injected {
				t.Fatal("capture IAM scope", policy)
			}
			if strings.Contains(m.resources["auth-password-set-enrollment"].Inputs["policy"].StringValue(), "arn:external-captures") {
				t.Fatal("password endpoint has capture access")
			}
		})
	}
}

func TestAuthenticationPreflightIgnoresCaptureCapacityAndRequiresPairedFacts(t *testing.T) {
	t.Setenv("PULUMI_CONFIG", `{"test:appOrigin":"https://dev.example.com","test:senderDomain":"example.com","test:senderAddress":"hello@example.com","test:captureReservedConcurrency":"invalid","test:captureWorkerMaxConcurrency":"invalid","test:environmentClass":"production"}`)
	dir := t.TempDir()
	for _, name := range requiredArchiveNames() {
		if err := os.WriteFile(filepath.Join(dir, name+".zip"), []byte("archive"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m := &backendMocks{associationMocks{resources: map[string]pulumi.MockResourceArgs{}}}
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		opts := Options{ArtifactDir: dir, RuntimeTableName: pulumi.String("runtime"), RuntimeTableARN: pulumi.String("arn:runtime"), Environment: "production", ResetFunctionName: "auth-reset-production"}
		if err := Validate(config.New(ctx, "test"), opts); err != nil {
			t.Fatal("auth checked card capacity", err)
		}
		opts.CaptureTableName = pulumi.String("external-captures")
		if err := Validate(config.New(ctx, "test"), opts); err == nil {
			t.Fatal("partial capture facts binding accepted")
		}
		opts.CaptureTableARN = pulumi.String("")
		if err := Validate(config.New(ctx, "test"), opts); err == nil {
			t.Fatal("empty capture facts binding accepted")
		}
		return nil
	}, pulumi.WithMocks("test", "production", m))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.resources) != 0 {
		t.Fatal("preflight registered resources", m.resources)
	}
}
