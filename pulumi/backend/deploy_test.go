package backend

import (
	"archive/zip"
	"encoding/json"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalSenderAndIsolatedRuntime(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "infra"), 0700)
	os.MkdirAll(filepath.Join(root, "dist"), 0700)
	names := []string{"signup", "resend", "confirm", "passkeyoptions", "passkeycomplete", "challenge", "auth-email-start", "auth-email-complete", "auth-passkey-start", "auth-passkey-complete", "auth-refresh", "auth-status", "auth-password-start", "auth-password-complete", "auth-password-set", "capture", "capture-worker", "capture-dispatcher", "app-config", "environment-reset"}
	for _, name := range names {
		f, err := os.Create(filepath.Join(root, "dist", name+".zip"))
		if err != nil {
			t.Fatal(err)
		}
		z := zip.NewWriter(f)
		w, _ := z.Create("bootstrap")
		w.Write([]byte("test"))
		z.Close()
		f.Close()
	}
	t.Chdir(filepath.Join(root, "infra"))
	cfg := map[string]string{"deploymentPrefix": "next", "environment": "dev", "expectedAccountId": "123456789012", "expectedRegion": "us-east-2", "sharedStack": "next-shared", "sharedAccountId": "123456789012", "sharedRegion": "us-east-2", "runtimeTableName": "runtime-next-shared", "runtimeTableArn": "arn:aws:dynamodb:us-east-2:123456789012:table/runtime-next-shared", "appOrigin": "https://next-dev.example.com", "senderDomain": "example.com", "senderAddress": "auth@example.com", "senderIdentityArn": "arn:aws:ses:us-east-2:123456789012:identity/example.com", "proofKey": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="}
	values := map[string]string{}
	for k, v := range cfg {
		values["attestra-backend:"+k] = v
	}
	b, _ := json.Marshal(values)
	t.Setenv("PULUMI_CONFIG", string(b))
	m := &associationMocks{resources: map[string]pulumi.MockResourceArgs{}}
	if err := pulumi.RunErr(Deploy, pulumi.WithMocks("attestra-backend", "next-dev", m)); err != nil {
		t.Fatal(err)
	}
	for _, r := range m.resources {
		if strings.HasPrefix(r.TypeToken, "aws:ses/") || strings.HasPrefix(r.TypeToken, "aws:route53/") {
			t.Fatalf("external sender resources owned: %s", r.TypeToken)
		}
	}
	reset := m.resources["environment-reset"].Inputs
	if reset["name"].StringValue() != "attestra-backend-reset-next-dev" {
		t.Fatal("reset name crosses deployment boundary")
	}
	for _, name := range []string{"app-config", "capture", "auth-status", "environment-reset"} {
		env := m.resources[name].Inputs["environment"].ObjectValue()["variables"].ObjectValue()
		if env["ENVIRONMENT"].StringValue() != "next-dev" {
			t.Fatalf("%s wrong runtime scope", name)
		}
	}
	if _, ok := m.resources["capture-queue-worker"]; !ok {
		t.Fatal("capture worker missing")
	}
}
