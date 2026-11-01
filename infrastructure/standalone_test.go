package infrastructure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func TestStandaloneRuntimeProfiles(t *testing.T) {
	for _, reuse := range []bool{false, true} {
		t.Run(map[bool]string{false: "isolated", true: "provided"}[reuse], func(t *testing.T) {
			testConfig(t)
			dir := t.TempDir()
			for _, name := range requiredArchiveNames() {
				if err := os.WriteFile(filepath.Join(dir, name+".zip"), []byte("archive"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			m := &backendMocks{associationMocks{resources: map[string]pulumi.MockResourceArgs{}}}
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				opts := Options{ArtifactDir: dir}
				if reuse {
					opts.RuntimeTableName = pulumi.String("provided-runtime")
					opts.RuntimeTableARN = pulumi.String("arn:provided-runtime")
				}
				d, err := DeployStandalone(ctx, config.New(ctx, "test"), opts)
				if err != nil {
					return err
				}
				if (d.RuntimeTable == nil) != reuse {
					t.Fatalf("unexpected runtime table ownership")
				}
				ctx.Export("table", d.RuntimeTableName)
				return nil
			}, pulumi.WithMocks("independent-project", "dev", m))
			if err != nil {
				t.Fatal(err)
			}
			table, exists := m.resources["auth-runtime-configuration"]
			if exists == reuse {
				t.Fatalf("unexpected table creation: %v", exists)
			}
			if !reuse {
				for key, want := range map[string]string{"billingMode": "PAY_PER_REQUEST", "hashKey": "id", "rangeKey": "sk"} {
					if table.Inputs[resource.PropertyKey(key)].StringValue() != want {
						t.Fatalf("bad %s", key)
					}
				}
				if !table.Inputs["deletionProtectionEnabled"].BoolValue() {
					t.Fatal("runtime table lacks deletion protection")
				}
				if !table.Inputs["serverSideEncryption"].ObjectValue()["enabled"].BoolValue() || !table.Inputs["pointInTimeRecovery"].ObjectValue()["enabled"].BoolValue() {
					t.Fatal("runtime table lacks encryption/PITR")
				}
			}
			if !strings.HasPrefix(m.resources["environment-reset"].Inputs["name"].StringValue(), "apexfission-auth-reset-independent-project-dev") {
				t.Fatal("unsafe reset default")
			}
		})
	}
}

func TestStandalonePreflightBeforeIsolatedTable(t *testing.T) {
	testConfig(t)
	for _, profile := range []string{"missing-archives", "provided-missing-archives", "partial-binding", "invalid-origin"} {
		t.Run(profile, func(t *testing.T) {
			m := &backendMocks{associationMocks{resources: map[string]pulumi.MockResourceArgs{}}}
			if profile == "invalid-origin" {
				t.Setenv("PULUMI_CONFIG", `{"test:appOrigin":"http://insecure.example.com"}`)
			}
			err := pulumi.RunErr(func(ctx *pulumi.Context) error {
				opts := Options{ArtifactDir: t.TempDir()}
				if profile == "provided-missing-archives" {
					opts.RuntimeTableName = pulumi.String("provided")
					opts.RuntimeTableARN = pulumi.String("arn:provided")
				}
				if profile == "partial-binding" {
					opts.RuntimeTableName = pulumi.String("one-sided")
				}
				_, err := DeployStandalone(ctx, config.New(ctx, "test"), opts)
				return err
			}, pulumi.WithMocks("test", "test", m))
			if err == nil {
				t.Fatal("expected preflight error")
			}
			for _, r := range m.resources {
				if strings.HasPrefix(r.TypeToken, "aws:") {
					t.Fatalf("registered %s before preflight", r.TypeToken)
				}
			}
		})
	}
}
