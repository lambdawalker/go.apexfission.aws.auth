package wizard

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPulumiFailurePreservesUsefulDiagnosticsAndRedactsSecrets(t *testing.T) {
	file := filepath.Join(t.TempDir(), "passphrase")
	os.WriteFile(file, []byte("file-passphrase\n"), 0600)
	env := []string{"PULUMI_CONFIG_PASSPHRASE=unlock-value", "PULUMI_CONFIG_PASSPHRASE_FILE=" + file, "AWS_SECRET_ACCESS_KEY=long-cloud-secret", "AWS_SESSION_TOKEN=session-value", "CLOUDFLARE_API_TOKEN=dns-token"}
	raw := []byte("error: resource violates plan: updatedAt differs; AccessDenied\n unlock-value file-passphrase long-cloud-secret session-value dns-token\nPROOF_KEY: abc-secret\npassword=another-secret\nAuthorization: Bearer bearer-value\n")
	got := pulumiFailureDetails(env, []string{"up", "--plan", "plan.json"}, raw)
	for _, want := range []string{"violates plan", "updatedAt", "AccessDenied"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing diagnostic %q: %s", want, got)
		}
	}
	for _, secret := range []string{"unlock-value", "file-passphrase", "long-cloud-secret", "session-value", "dns-token", "abc-secret", "another-secret", "bearer-value"} {
		if strings.Contains(got, secret) {
			t.Errorf("secret exposed: %s", secret)
		}
	}
	for _, args := range [][]string{{"config", "--json"}, {"stack", "export"}, {"up", "--show-secrets"}} {
		if pulumiFailureDetails(env, args, raw) != "" {
			t.Fatal("sensitive command output exposed")
		}
	}
}

func TestFailedPulumiProcessExposesOnlyRedactedDeploymentDiagnostics(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture; redaction tests are platform independent")
	}
	root := t.TempDir()
	script := filepath.Join(root, "pulumi")
	if e := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' 'error: resource violates plan: updatedAt' 'AWS_SESSION_TOKEN=child-token'\nexit 1\n"), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AWS_SESSION_TOKEN", "child-token")
	_, err := (process{}).Run(context.Background(), root, os.Environ(), "pulumi", "up", "--plan", "reviewed.json")
	if err == nil || !strings.Contains(err.Error(), "violates plan") || strings.Contains(err.Error(), "child-token") {
		t.Fatalf("unsafe or missing diagnostic: %v", err)
	}
	_, err = (process{}).Run(context.Background(), root, os.Environ(), "pulumi", "config", "--json")
	if err == nil || strings.Contains(err.Error(), "violates plan") {
		t.Fatalf("config output exposed: %v", err)
	}
}
