package infrastructure

import (
	"github.com/lambdawalker/go.apexfission.aws.auth/router"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"strings"
	"sync"
	"testing"
)

type mocks struct {
	mu        sync.Mutex
	types     []string
	resources []pulumi.MockResourceArgs
}

func (m *mocks) Call(a pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{"region": resource.NewStringProperty("us-east-1"), "name": resource.NewStringProperty("us-east-1"), "accountId": resource.NewStringProperty("123456789012")}, nil
}
func (m *mocks) NewResource(a pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.types = append(m.types, a.TypeToken)
	m.resources = append(m.resources, a)
	out := a.Inputs.Copy()
	out["arn"] = resource.NewStringProperty("arn:aws:test:us-east-1:123456789012:" + a.Name)
	out["invokeArn"] = out["arn"]
	out["executionArn"] = out["arn"]
	out["endpoint"] = resource.NewStringProperty("cognito-idp.eu-west-1.amazonaws.com/pool")
	out["apiEndpoint"] = resource.NewStringProperty("https://api.example")
	return a.Name + "-id", out, nil
}
func TestAPIRejectsMissingIntegration(t *testing.T) {
	m := &mocks{}
	e := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, e := NewAPI(ctx, "sales", APIArgs{Routes: []router.Route{{Method: "GET", Path: "/orders", Lambda: "missing"}}})
		return e
	}, pulumi.WithMocks("sales", "dev", m))
	if e == nil {
		t.Fatal("missing integration accepted")
	}
}
func TestProtectedTableAndAlias(t *testing.T) {
	m := &mocks{}
	e := pulumi.RunErr(func(ctx *pulumi.Context) error {
		_, e := NewTable(ctx, "orders", "id", pulumi.Aliases([]pulumi.Alias{{Name: pulumi.String("old-orders"), NoParent: pulumi.Bool(true)}}))
		return e
	}, pulumi.WithMocks("sales", "dev", m))
	if e != nil {
		t.Fatal(e)
	}
	if len(m.resources) != 1 || !m.resources[0].RegisterRPC.GetProtect() {
		t.Fatal("stateful table not protected")
	}
	rpc := m.resources[0].RegisterRPC
	if len(rpc.GetAliases())+len(rpc.GetAliasURNs()) == 0 {
		t.Fatal("migration alias not forwarded")
	}
	if len(m.types) != 1 || m.types[0] != "aws:dynamodb/table:Table" {
		t.Fatal(m.types)
	}
}

func TestProtectedIntegrationRequiresRuntimeContract(t *testing.T) {
	m := &mocks{}
	e := pulumi.RunErr(func(ctx *pulumi.Context) error {
		fn, e := NewFunction(ctx, "handler", FunctionArgs{Archive: "handler.zip"})
		if e != nil {
			return e
		}
		_, e = NewAPI(ctx, "api", APIArgs{Issuer: pulumi.String("https://issuer"), ClientID: pulumi.String("client"), Functions: map[string]*lambda.Function{"handler": fn}, Routes: []router.Route{{Method: "GET", Path: "/orders", Lambda: "handler", Permission: "sales:read"}}})
		return e
	}, pulumi.WithMocks("sales", "dev", m))
	if e == nil || !strings.Contains(e.Error(), "explicitly enforce") {
		t.Fatal(e)
	}
}
func TestIssuerComesFromProvisionedPool(t *testing.T) {
	m := &mocks{}
	e := pulumi.RunErr(func(ctx *pulumi.Context) error {
		a, e := NewAuth(ctx, "auth", AuthArgs{Origin: "https://sales.example", SenderDomain: "mail.sales.example", SenderAddress: "hello@mail.sales.example", ChallengeArchive: "challenge.zip", ProofKey: pulumi.String("secret")})
		if e != nil {
			return e
		}
		a.Issuer.ApplyT(func(issuer string) string {
			if issuer != "https://cognito-idp.eu-west-1.amazonaws.com/pool" {
				t.Errorf("wrong provider issuer %s", issuer)
			}
			return issuer
		})
		return nil
	}, pulumi.WithMocks("sales", "dev", m))
	if e != nil {
		t.Fatal(e)
	}
}
