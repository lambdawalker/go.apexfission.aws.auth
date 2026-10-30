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

// prepareGitHubAuthorization collects first-time OIDC setup inputs. Unattended
// callers retain deterministic role discovery and fail closed in reconciliation.
func (x *execution) prepareGitHubAuthorization(interactive bool) error {
	needShared := x.c.Scope != "backend" && x.c.GitHubSharedRoleARN == ""
	needBackend := x.c.Scope != "shared" && x.c.GitHubBackendRoleARN == ""
	if !interactive || x.c.GitHubPolicy != "" || (!needShared && !needBackend) {
		return nil
	}
	read := func(prompt string) (string, error) {
		fmt.Fprint(x.o.Output, prompt)
		line, err := x.o.reader.ReadString('\n')
		if err != nil {
			return "", errors.New("GitHub authorization selection cancelled; no setup was performed")
		}
		return strings.TrimSpace(line), nil
	}
	fmt.Fprintln(x.o.Output, "GitHub Actions needs an AWS deployment role for each target. New roles require a reviewed IAM policy JSON; existing roles must trust this repository and GitHub environment exactly.")
	for {
		mode, err := read("GitHub AWS authorization (policy/roles/skip) [skip]: ")
		if err != nil {
			return err
		}
		switch strings.ToLower(mode) {
		case "", "skip":
			x.c.GitHubSetup = false
			x.c.GitHubAsked = true
			fmt.Fprintln(x.o.Output, "GitHub setup deferred. Local deployment can continue; run --operation setup-github later.")
			return nil
		case "policy":
			path, err := read("Reviewed deployment IAM policy JSON file: ")
			if err != nil {
				return err
			}
			path = strings.Trim(path, "\"")
			if !filepath.IsAbs(path) {
				path = filepath.Join(x.o.Root, path)
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read reviewed GitHub policy: %w", err)
			}
			if !json.Valid(b) {
				return errors.New("reviewed GitHub policy file is not valid JSON")
			}
			x.c.GitHubPolicy = path
			return nil
		case "roles":
			// Commit both answers together so cancellation cannot save partial input.
			shared, backend := x.c.GitHubSharedRoleARN, x.c.GitHubBackendRoleARN
			for _, target := range []struct {
				needed bool
				label  string
				value  *string
			}{
				{needShared, "Admin/index", &shared}, {needBackend, "Backend", &backend},
			} {
				if !target.needed {
					continue
				}
				role, err := read(target.label + " existing deployment role ARN: ")
				if err != nil {
					return err
				}
				if role == "" {
					return errors.New("existing role ARN is required; use skip to defer GitHub setup")
				}
				*target.value = role
			}
			x.c.GitHubSharedRoleARN, x.c.GitHubBackendRoleARN = shared, backend
			return nil
		default:
			fmt.Fprintln(x.o.Output, "Choose policy, roles, or skip.")
		}
	}
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
	authorization := "existing repository/environment-scoped roles"
	if x.c.GitHubPolicy != "" {
		authorization = "reviewed policy " + x.c.GitHubPolicy
	}
	fmt.Fprintf(x.o.Output, "Configuring GitHub targets for %s using %s (existing protections preserved; no workflow is dispatched)\n", x.c.GitHubRepository, authorization)
	setup := x.o.SetupGitHub
	if setup == nil {
		setup = githubsetup.Reconcile
	}
	return setup(x.ctx, githubsetup.Options{Root: x.o.Root, Repository: x.c.GitHubRepository, Account: x.c.Account, Region: x.c.Region, PolicyFile: x.c.GitHubPolicy, Workflow: ".github/workflows/deploy.yml", WorkflowFiles: []string{".github/workflows/deploy-target.yml"}, Targets: targets, SecretStateKey: x.secretMetadataKey, Runner: x.o.Runner, Env: x.env, Output: x.o.Output, Force: x.c.GitHubForce, RotateSecrets: x.c.GitHubRotate, Lock: x.acquireLock})
}
