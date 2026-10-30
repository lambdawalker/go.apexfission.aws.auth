package wizard

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lambdawalker/go.apexfission.aws.auth/deploy/githubsetup"
)

func envValue(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], key+"=") {
			return strings.TrimPrefix(env[i], key+"=")
		}
	}
	return ""
}
func (x *execution) setupGitHub() error {
	if x.c.GitHubRepository == "" {
		b, e := x.run("gh", "repo", "view", "--json", "nameWithOwner")
		if e != nil {
			return errors.New("set --github-repository owner/repo and authenticate with gh auth login")
		}
		var repo struct{ NameWithOwner string }
		if json.Unmarshal(b, &repo) != nil || repo.NameWithOwner == "" {
			return errors.New("cannot resolve target GitHub repository")
		}
		x.c.GitHubRepository = repo.NameWithOwner
	}
	passphrase := envValue(x.env, "PULUMI_CONFIG_PASSPHRASE")
	if passphrase == "" {
		if file := envValue(x.env, "PULUMI_CONFIG_PASSPHRASE_FILE"); file != "" {
			b, e := os.ReadFile(file)
			if e != nil {
				return e
			}
			passphrase = strings.TrimSpace(string(b))
			clear(b)
		}
	}
	if passphrase == "" {
		return errors.New("GitHub setup requires the Pulumi passphrase from the unlocked vault or protected environment input")
	}
	if x.c.GitHubSecretVersion == "" {
		x.c.GitHubSecretVersion = "1"
	}
	targets := []githubsetup.Target{}
	for _, shared := range []bool{true, false} {
		if shared && x.c.Scope == "backend" || !shared && x.c.Scope == "shared" {
			continue
		}
		c := x.c
		c.Operation = "preview"
		c.GitHubSetup = false
		c.GitHubAsked = true
		c.Profile = ""
		c.GitHubForce = false
		c.GitHubRotate = false
		c.ForceRebuild = false
		c.CleanCache = false
		role := c.GitHubBackendRoleARN
		c.Scope = "backend"
		if shared {
			c.Scope = "shared"
			role = c.GitHubSharedRoleARN
		}
		snapshot := saved{Version: 1, Choices: c, Bindings: map[string]binding{c.identity(): {Account: c.Account, Region: c.Region, StateBackend: c.StateBackend}}}
		b, e := json.Marshal(snapshot)
		if e != nil {
			return e
		}
		variables := map[string]string{"AWS_ACCOUNT_ID": c.Account, "AWS_REGION": c.Region, "PULUMI_BACKEND_URL": c.StateBackend, "BASE_DOMAIN": c.BaseDomain, "WIZARD_SETTINGS_JSON": string(b)}
		secrets := map[string]string{"PULUMI_CONFIG_PASSPHRASE": passphrase}
		if c.Route53ZoneID == "" {
			if x.cloudflareToken == "" {
				return errors.New("GitHub Cloudflare deployments need a saved/explicit Cloudflare token; alternatively configure Route53")
			}
			secrets["CLOUDFLARE_API_TOKEN"] = x.cloudflareToken
		}
		targets = append(targets, githubsetup.Target{Environment: c.stack(shared), RoleARN: role, SecretVersion: c.GitHubSecretVersion, Variables: variables, Secrets: secrets})
	}
	// Preserve reviewed nonsecret answers even if remote setup is interrupted.
	path := filepath.Join(x.o.Root, ".attestra", "wizard.json")
	state := saved{Version: 1}
	if b, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(b, &state) != nil || state.Version != 1 {
			return errors.New("invalid local settings; refusing to overwrite them")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := state.bind(x.c); err != nil {
		return err
	}
	state.remember(x.c)
	if err := save(path, state); err != nil {
		return err
	}
	fmt.Fprintf(x.o.Output, "Configuring GitHub targets for %s using reviewed policy %s (existing protections preserved; no workflow is dispatched)\n", x.c.GitHubRepository, x.c.GitHubPolicy)
	setup := x.o.SetupGitHub
	if setup == nil {
		setup = githubsetup.Reconcile
	}
	return setup(x.ctx, githubsetup.Options{Root: x.o.Root, Repository: x.c.GitHubRepository, Account: x.c.Account, Region: x.c.Region, PolicyFile: x.c.GitHubPolicy, Workflow: ".github/workflows/deploy.yml", WorkflowFiles: []string{".github/workflows/deploy-target.yml"}, Targets: targets, SecretStateKey: x.secretMetadataKey, Runner: x.o.Runner, Env: x.env, Output: x.o.Output, Force: x.c.GitHubForce, RotateSecrets: x.c.GitHubRotate, Lock: x.acquireLock})
}
