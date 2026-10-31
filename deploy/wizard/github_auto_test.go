package wizard

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"github.com/lambdawalker/go.apexfission.aws.auth/deploy/githubsetup"
	"io"
	"regexp"
	"strings"
	"testing"
)

func autoExecution(t *testing.T, input string) *execution {
	o, _ := fixture(t)
	o.GitHubPolicyProfile = githubsetup.AttestraProfile
	o.Output = &bytes.Buffer{}
	o.reader = bufio.NewReader(strings.NewReader(input))
	_, c, _, e := parse(o)
	if e != nil {
		t.Fatal(e)
	}
	c.GitHubRepository = "owner/repo"
	c.SenderDomain = "dev.info.example.com"
	c.SenderAddress = "verify@dev.info.example.com"
	c.GitHubAutomatic = true
	return &execution{ctx: context.Background(), o: o, c: c}
}
func TestAutomaticGitHubApprovalAndReuse(t *testing.T) {
	x := autoExecution(t, "yes\n")
	if e := x.prepareAutomaticGitHub(true); e != nil {
		t.Fatal(e)
	}
	if x.c.GitHubSharedApproval == "" || x.c.GitHubBackendApproval == "" || len(x.generated) != 2 {
		t.Fatal("approval/pins not saved")
	}
	x.o.reader = bufio.NewReader(strings.NewReader(""))
	x.o.Output = io.Discard
	if e := x.prepareAutomaticGitHub(true); e != nil {
		t.Fatal("unchanged approval prompted:", e)
	}
	x.c.Region = "us-west-2"
	if e := x.prepareAutomaticGitHub(true); e == nil {
		t.Fatal("changed permissions silently approved")
	}
}
func TestAutomaticGitHubRequiresExplicitPermissionApproval(t *testing.T) {
	for _, input := range []string{"no\n", "", "\n"} {
		x := autoExecution(t, input)
		if e := x.prepareAutomaticGitHub(true); e == nil {
			t.Fatalf("approved %q", input)
		}
		if x.c.GitHubSharedApproval != "" || x.c.GitHubBackendApproval != "" {
			t.Fatal("rejected policy remembered")
		}
	}
	x := autoExecution(t, "")
	x.approved = true
	if e := x.prepareAutomaticGitHub(false); e == nil {
		t.Fatal("--yes approved new IAM permissions")
	}
}
func TestProfileSelectsAutomaticWithoutTechnicalQuestion(t *testing.T) {
	x := autoExecution(t, "")
	x.c.GitHubAutomatic = false
	x.c.GitHubSetup = true
	if e := x.prepareGitHubAuthorization(true); e != nil || !x.c.GitHubAutomatic {
		t.Fatalf("automatic profile not selected: %v", e)
	}
}

func TestAutomaticSetupPersistsScopedCISnapshots(t *testing.T) {
	o, r := fixture(t)
	o.GitHubPolicyProfile = githubsetup.AttestraProfile
	t.Setenv("CLOUDFLARE_API_TOKEN", "dns-secret")
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "pulumi-secret")
	o.Args = append(o.Args, "--operation", "setup-github", "--yes", "--github-repository", "owner/repo", "--sender-domain", "dev.info.example.com", "--sender-address", "verify@dev.info.example.com")
	calls := 0
	o.SetupGitHub = func(_ context.Context, opt githubsetup.Options) error {
		calls++
		for _, target := range opt.Targets {
			if target.Generated == nil {
				t.Fatal("automatic setup did not deliver generated policies")
			}
			var state saved
			if e := json.Unmarshal([]byte(target.Variables["WIZARD_SETTINGS_JSON"]), &state); e != nil {
				t.Fatal(e)
			}
			if !state.Choices.GitHubAutomatic || state.Choices.GitHubSetup || state.Choices.GitHubApprovalToken != "" {
				t.Fatal("unsafe CI settings")
			}
			if state.Choices.Scope == "shared" && state.Choices.GitHubBackendApproval != "" {
				t.Fatal("shared snapshot contains backend approval")
			}
		}
		return nil
	}
	err := Run(context.Background(), o)
	if err == nil || calls != 0 {
		t.Fatalf("new IAM permissions enrolled without explicit approval: %v", err)
	}
	match := regexp.MustCompile(`--github-approve-policy ([a-f0-9]{64})`).FindStringSubmatch(err.Error())
	if len(match) != 2 {
		t.Fatal(err)
	}
	for _, call := range r.calls {
		if strings.Contains(call, "create-role") || strings.Contains(call, "create-policy") {
			t.Fatal("mutation before approval")
		}
	}
	o.Args = append(o.Args, "--github-approve-policy", match[1])
	if err := Run(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("setup did not complete")
	}
	// Saved approvals suffice on retry without the invocation-only approval token.
	o.Args = o.Args[:len(o.Args)-2]
	if err := Run(context.Background(), o); err != nil {
		t.Fatal(err)
	}
}

func TestSharedSnapshotIndependentOfBackendFeatures(t *testing.T) {
	x := autoExecution(t, "yes\n")
	if e := x.prepareAutomaticGitHub(true); e != nil {
		t.Fatal(e)
	}
	x.env = withEnv(nil, "PULUMI_CONFIG_PASSPHRASE", "passphrase")
	x.cloudflareToken = "dns-token"
	var snapshots []string
	x.o.SetupGitHub = func(_ context.Context, o githubsetup.Options) error {
		for _, target := range o.Targets {
			if target.Environment == "next-shared" {
				snapshots = append(snapshots, target.Variables["WIZARD_SETTINGS_JSON"])
			}
		}
		return nil
	}
	if e := x.setupGitHub(false); e != nil {
		t.Fatal(e)
	}
	x.c.CaptureReserved = 5
	x.c.CaptureWorker = 5
	x.c.CaptureEnabled = true
	x.c.EnvironmentClass = "testing"
	if e := x.setupGitHub(false); e != nil {
		t.Fatal(e)
	}
	if len(snapshots) != 2 || snapshots[0] != snapshots[1] {
		t.Fatal("backend features changed shared GitHub settings")
	}
}
