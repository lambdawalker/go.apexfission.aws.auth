package deploy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestProviderOverrideFailsBeforePreview(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "infra"), 0755)
	m := Manifest{Version: 1, Application: "sales", Modules: map[string]string{"auth": "0.1.0"}, Environments: map[string]Environment{"dev": {Account: "123456789012", Region: "us-east-1", Backend: "s3://state", Stack: "dev", Directory: "infra"}}}
	r := &recorder{}
	if e := (Engine{Manifest: m, Root: root, Runner: r}).Execute(context.Background(), "preview", "dev", Options{}); e == nil {
		t.Fatal("missing provider account restriction accepted")
	}
}
func TestStackAliasRejected(t *testing.T) {
	m := Manifest{Version: 1, Application: "sales", Modules: map[string]string{"auth": "0.1.0"}, Environments: map[string]Environment{"dev": {Account: "123456789012", Region: "us-east-1", Backend: "s3://state", Stack: "prod", Directory: "infra"}}}
	if e := m.Validate(); e == nil {
		t.Fatal("production stack hidden behind dev")
	}
}
