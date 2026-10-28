package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"sync"
	"testing"
)

type mocks struct {
	mu    sync.Mutex
	types []string
}

func (m *mocks) Call(a pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return resource.PropertyMap{"region": resource.NewStringProperty("us-east-1"), "name": resource.NewStringProperty("us-east-1"), "accountId": resource.NewStringProperty("123456789012")}, nil
}
func (m *mocks) NewResource(a pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.types = append(m.types, a.TypeToken)
	out := a.Inputs.Copy()
	out["arn"] = resource.NewStringProperty("arn:aws:test:us-east-1:123456789012:" + a.Name)
	out["invokeArn"] = out["arn"]
	out["executionArn"] = out["arn"]
	out["endpoint"] = resource.NewStringProperty("cognito-idp.us-east-1.amazonaws.com/pool")
	out["apiEndpoint"] = resource.NewStringProperty("https://api.example")
	return a.Name + "-id", out, nil
}
func TestSalesStackComposition(t *testing.T) {
	m := &mocks{}
	e := pulumi.RunErr(func(ctx *pulumi.Context) error {
		return compose(ctx, "https://sales.example", "https://admin.sales.example", "mail.sales.example", "hello@mail.sales.example", pulumi.String("c2VjcmV0"), t.TempDir())
	}, pulumi.WithMocks("sales", "dev", m))
	if e != nil {
		t.Fatal(e)
	}
	counts := map[string]int{}
	for _, kind := range m.types {
		counts[kind]++
	}
	if counts["aws:dynamodb/table:Table"] != 3 || counts["aws:cognito/userPool:UserPool"] != 2 || counts["aws:lambda/function:Function"] != 4 || counts["aws:apigatewayv2/route:Route"] < 14 {
		t.Fatal(counts)
	}
}
