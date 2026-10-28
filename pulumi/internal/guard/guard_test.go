package guard

import (
	"encoding/json"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"strings"
	"testing"
)

type mocks struct {
	account, region string
	resources       int
}

func (m *mocks) Call(a pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.NewPropertyMapFromMap(map[string]any{"accountId": m.account, "name": m.region}), nil
}
func (m *mocks) NewResource(a pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	m.resources++
	return a.Name, a.Inputs, nil
}
func TestIdentityPins(t *testing.T) {
	for _, tt := range []struct {
		name, project, stack, prefix, account, region, shared string
		valid                                                 bool
	}{
		{"hyphen-prefix", "attestra-backend", "a-b-dev", "a-b", "123456789012", "us-east-2", "a-b-shared", false},
		{"double-hyphen-env", "attestra-backend", "next-a--b", "next", "123456789012", "us-east-2", "next-shared", false},
		{"trailing-hyphen-env", "attestra-backend", "next-dev-", "next", "123456789012", "us-east-2", "next-shared", false},
		{"long-env", "attestra-backend", "next-abcdefghijklmnopqrstuvwxyzabcdef", "next", "123456789012", "us-east-2", "next-shared", false},
		{"next", "attestra-backend", "next-dev", "next", "123456789012", "us-east-2", "next-shared", true},
		{"empty-is-final", "attestra-backend", "final-dev", "", "123456789012", "us-east-2", "final-shared", true},
		{"legacy", "attestra-auth-email", "dev", "", "123456789012", "us-east-2", "final-shared", false},
		{"wrong-stack", "attestra-backend", "dev", "", "123456789012", "us-east-2", "final-shared", false},
		{"wrong-account", "attestra-backend", "next-dev", "next", "999999999999", "us-east-2", "next-shared", false},
		{"wrong-region", "attestra-backend", "next-dev", "next", "123456789012", "us-west-2", "next-shared", false},
		{"wrong-shared", "attestra-backend", "next-dev", "next", "123456789012", "us-east-2", "final-shared", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := map[string]string{"deploymentPrefix": tt.prefix, "environment": strings.TrimPrefix(tt.stack, tt.prefix+"-"), "expectedAccountId": "123456789012", "expectedRegion": "us-east-2", "sharedStack": tt.shared, "sharedAccountId": "123456789012", "sharedRegion": "us-east-2", "runtimeTableArn": "arn:aws:dynamodb:us-east-2:123456789012:table/runtime"}
			for _, key := range []string{"runtimeTableName", "appOrigin", "senderDomain", "senderAddress", "senderIdentityArn", "proofKey"} {
				cfg[key] = "test"
			}
			if tt.prefix == "" {
				cfg["environment"] = "dev"
			}
			cfg["runtimeTableName"] = "runtime"
			values := map[string]string{}
			for k, v := range cfg {
				values[tt.project+":"+k] = v
			}
			b, _ := json.Marshal(values)
			t.Setenv("PULUMI_CONFIG", string(b))
			m := &mocks{account: tt.account, region: tt.region}
			err := pulumi.RunErr(func(ctx *pulumi.Context) error { return Check(ctx, "backend") }, pulumi.WithMocks(tt.project, tt.stack, m))
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v got %v", tt.valid, err)
			}
			if m.resources != 0 {
				t.Fatal("guard registered resources")
			}
		})
	}
}

func TestRequiredExplicitPrefixAndMatchedRuntimeTable(t *testing.T) {
	for _, scope := range []string{"backend", "shared"} {
		for _, kind := range []string{"missing-prefix", "explicit-empty", "mismatched-table"} {
			if scope == "shared" && kind == "mismatched-table" {
				continue
			}
			t.Run(scope+"/"+kind, func(t *testing.T) {
				cfg := map[string]string{"deploymentPrefix": "", "environment": "dev", "expectedAccountId": "123456789012", "expectedRegion": "us-east-2", "sharedStack": "final-shared", "sharedAccountId": "123456789012", "sharedRegion": "us-east-2", "runtimeTableName": "runtime", "runtimeTableArn": "arn:aws:dynamodb:us-east-2:123456789012:table/runtime", "appOrigin": "https://example.com", "senderDomain": "example.com", "senderAddress": "auth@example.com", "senderIdentityArn": "arn:aws:ses:us-east-2:123456789012:identity/example.com", "proofKey": "test", "domain": "index.example.com", "adminDomain": "admin.example.com", "sourceHash": "test"}
				if kind == "missing-prefix" {
					delete(cfg, "deploymentPrefix")
				}
				if kind == "mismatched-table" {
					cfg["runtimeTableName"] = "wrong-table"
				}
				project := "attestra-" + scope
				stack := "final-shared"
				if scope == "backend" {
					stack = "final-dev"
				}
				values := map[string]string{}
				for k, v := range cfg {
					values[project+":"+k] = v
				}
				b, _ := json.Marshal(values)
				t.Setenv("PULUMI_CONFIG", string(b))
				m := &mocks{account: "123456789012", region: "us-east-2"}
				err := pulumi.RunErr(func(ctx *pulumi.Context) error { return Check(ctx, scope) }, pulumi.WithMocks(project, stack, m))
				if (err == nil) != (kind == "explicit-empty") {
					t.Fatalf("unexpected result: %v", err)
				}
				if kind == "missing-prefix" && !strings.Contains(err.Error(), "explicitly configured") {
					t.Fatal(err)
				}
				if kind == "mismatched-table" && !strings.Contains(err.Error(), "match runtimeTableName") {
					t.Fatal(err)
				}
				if m.resources != 0 {
					t.Fatal("guard registered resources")
				}
			})
		}
	}
}
