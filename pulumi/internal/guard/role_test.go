package guard

import (
	"encoding/json"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"sync"
	"testing"
)

type roleMocks struct {
	mu     sync.Mutex
	inputs map[string]resource.PropertyMap
}

func (m *roleMocks) Call(pulumi.MockCallArgs) (resource.PropertyMap, error) { return nil, nil }
func (m *roleMocks) NewResource(a pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	m.mu.Lock()
	m.inputs[a.Name] = a.Inputs
	m.mu.Unlock()
	return a.Name, a.Inputs, nil
}
func TestAutomaticRuntimeRoleAndFunction(t *testing.T) {
	for _, auto := range []bool{false, true} {
		cfg := map[string]string{"attestra-backend:expectedAccountId": "123456789012"}
		if auto {
			cfg["attestra-backend:deploymentRolePrefix"] = "af-0123456789abcdef-"
			cfg["attestra-backend:deploymentBoundaryArn"] = "arn:aws:iam::123456789012:policy/attestra-runtime-0123456789abcdef01234567"
		}
		b, _ := json.Marshal(cfg)
		t.Setenv("PULUMI_CONFIG", string(b))
		m := &roleMocks{inputs: map[string]resource.PropertyMap{}}
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			if _, e := Role(ctx, "capture-role", &iam.RoleArgs{AssumeRolePolicy: pulumi.String("{}")}); e != nil {
				return e
			}
			if _, e := Function(ctx, "capture", &lambda.FunctionArgs{Role: pulumi.String("role"), Runtime: pulumi.String("provided.al2023"), Handler: pulumi.String("bootstrap")}); e != nil {
				return e
			}
			_, e := Function(ctx, "reset", &lambda.FunctionArgs{Role: pulumi.String("role"), Runtime: pulumi.String("provided.al2023"), Handler: pulumi.String("bootstrap"), Name: pulumi.String("attestra-backend-reset-next-dev")})
			return e
		}, pulumi.WithMocks("attestra-backend", "next-dev", m))
		if err != nil {
			t.Fatal(err)
		}
		if auto {
			if m.inputs["capture-role"]["name"].StringValue() != "af-0123456789abcdef-capture-role" || m.inputs["capture-role"]["permissionsBoundary"].StringValue() != cfg["attestra-backend:deploymentBoundaryArn"] || m.inputs["capture"]["name"].StringValue() != "af-0123456789abcdef-capture" {
				t.Fatal("runtime escaped automatic IAM pins")
			}
		} else if _, ok := m.inputs["capture-role"]["permissionsBoundary"]; ok {
			t.Fatal("manual role changed")
		}
		if m.inputs["reset"]["name"].StringValue() != "attestra-backend-reset-next-dev" {
			t.Fatal("reset routing changed")
		}
	}
}
