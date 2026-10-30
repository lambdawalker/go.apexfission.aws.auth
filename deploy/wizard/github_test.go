package wizard

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lambdawalker/go.apexfission.aws.auth/deploy/githubsetup"
)

func TestGitHubSetupExportsReviewedScopeWithoutSecretSettings(t *testing.T) {
	o, r := fixture(t)
	t.Setenv("CLOUDFLARE_API_TOKEN", "dns-secret-value")
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "pulumi-secret-value")
	o.Args = append(o.Args, "--operation", "setup-github", "--yes", "--github-repository", "owner/repo")
	called := false
	o.SetupGitHub = func(ctx context.Context, opt githubsetup.Options) error {
		called = true
		if opt.Repository != "owner/repo" || len(opt.Targets) != 2 || opt.Lock == nil {
			t.Fatalf("invalid setup request: %+v", opt)
		}
		for _, target := range opt.Targets {
			if strings.Contains(target.Variables["WIZARD_SETTINGS_JSON"], "dns-secret-value") {
				t.Fatal("secret in public config")
			}
			var s saved
			if json.Unmarshal([]byte(target.Variables["WIZARD_SETTINGS_JSON"]), &s) != nil {
				t.Fatal("invalid snapshot")
			}
			expected := "backend"
			if strings.HasSuffix(target.Environment, "-shared") {
				expected = "shared"
			}
			if expected == "shared" {
				root := t.TempDir()
				if err := save(filepath.Join(root, ".attestra", "wizard.json"), s); err != nil {
					t.Fatal(err)
				}
				_, got, _, err := parse(Options{Root: root, Output: io.Discard, Args: []string{"--scope", "shared", "--environment", "other", "--operation", "build"}})
				if err != nil || got.Account != s.Choices.Account || got.StateBackend != s.Choices.StateBackend || got.Environment != s.Choices.Environment {
					t.Fatalf("shared workflow lost snapshot: %+v %v", got, err)
				}
			}
			if expected == "backend" {
				root := t.TempDir()
				if err := save(filepath.Join(root, ".attestra", "wizard.json"), s); err != nil {
					t.Fatal(err)
				}
				_, got, _, err := parse(Options{Root: root, Output: io.Discard, Args: []string{"--scope", "backend", "--environment", "other", "--operation", "build"}})
				if err != nil || got.Account != "" || got.StateBackend != "" || got.Environment != "other" {
					t.Fatalf("backend snapshot crossed environment boundary: %+v %v", got, err)
				}
			}
			if s.Choices.Scope != expected || s.Choices.Profile != "" || s.Choices.GitHubSetup {
				t.Fatalf("unsafe CI choices: %+v", s.Choices)
			}
			if target.Secrets["PULUMI_CONFIG_PASSPHRASE"] == "" || target.Secrets["CLOUDFLARE_API_TOKEN"] != "dns-secret-value" {
				t.Fatal("missing protected secrets")
			}
		}
		return nil
	}
	if err := Run(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("setup not called")
	}
	for _, call := range r.calls {
		if strings.Contains(call, "pulumi up") {
			t.Fatal("setup deployed infrastructure")
		}
	}
}
