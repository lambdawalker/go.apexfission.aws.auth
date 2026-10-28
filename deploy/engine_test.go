package deploy

import (
	"context"
	"strings"
	"testing"
)

type recorder struct{ calls []string }

func (r *recorder) Run(_ context.Context, _ string, _ []string, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	if name == "aws" {
		return []byte(`{"Account":"123456789012"}`), nil
	}
	return []byte(`{}`), nil
}
func TestTeardownRequiresExactTarget(t *testing.T) {
	m := Manifest{Version: 1, Application: "sales", Modules: map[string]string{"auth": "0.1.0"}, Environments: map[string]Environment{"prod": {Account: "123456789012", Region: "us-east-1", Backend: "s3://sales-state", Stack: "prod", Directory: "infra", Production: true}}}
	r := &recorder{}
	e := Engine{Manifest: m, Root: t.TempDir(), Runner: r}
	if err := e.Execute(context.Background(), "teardown", "prod", Options{}); err == nil || len(r.calls) != 0 {
		t.Fatal("unguarded teardown", err, r.calls)
	}
}
