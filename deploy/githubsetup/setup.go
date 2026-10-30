// Package githubsetup reconciles protected deployment environments without dispatching workflows.
package githubsetup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	deploy "github.com/lambdawalker/go.apexfission.aws.auth/deploy"
	gh "github.com/lambdawalker/go.apexfission.aws.auth/deploy/github"
)

const Marker = "DEPLOY_CONFIG_SHA256"

type Target struct {
	Environment, RoleARN, SecretVersion string
	Variables                           map[string]string
	Secrets                             map[string]string
}
type Options struct {
	Root, Repository, Account, Region, PolicyFile, Workflow string
	WorkflowFiles                                           []string
	Targets                                                 []Target
	SecretStateKey                                          []byte
	Runner                                                  deploy.Runner
	Env                                                     []string
	Output                                                  io.Writer
	Force, RotateSecrets                                    bool
	// Lock must serialize the same deployment identity across machines (the wizard uses S3).
	Lock func() (func(), error)
}
type inputRunner interface {
	RunInput(context.Context, string, []string, io.Reader, string, ...string) ([]byte, error)
}
type client struct {
	ctx   context.Context
	o     Options
	stdin inputRunner
}

func (c client) run(result any, name string, args ...string) error {
	b, e := c.o.Runner.Run(c.ctx, c.o.Root, c.o.Env, name, args...)
	if e != nil {
		return fmt.Errorf("%s setup failed; check authenticated identity, repository environment administration, variables/secrets and IAM permissions; completed changes retained: %w", name, e)
	}
	if result != nil && json.Unmarshal(b, result) != nil {
		return errors.New("invalid setup response")
	}
	return nil
}
func (c client) api(method, path string, body, result any) error {
	args := []string{"api", "--method", method, path}
	var b []byte
	var e error
	if body != nil {
		b, e = json.Marshal(body)
		if e != nil {
			return e
		}
		args = append(args, "--input", "-")
	}
	out, e := c.stdin.RunInput(c.ctx, c.o.Root, c.o.Env, bytes.NewReader(b), "gh", args...)
	if e != nil {
		return fmt.Errorf("GitHub environment setup incomplete (%s %s); verify gh auth status and repository Administration/Environments/Variables/Secrets permissions; rerun setup to reconcile: %w", method, path, e)
	}
	if result != nil && json.Unmarshal(out, result) != nil {
		return errors.New("invalid GitHub setup response")
	}
	return nil
}
func (c client) pages(path string, result any) error {
	return c.run(result, "gh", "api", "--paginate", "--slurp", path+"?per_page=100")
}
func Fingerprint(repo string, t Target, workflow string) (string, error) {
	names := make([]string, 0, len(t.Secrets))
	for n := range t.Secrets {
		names = append(names, n)
	}
	sort.Strings(names)
	vars := map[string]string{}
	for k, v := range t.Variables {
		if k != Marker {
			vars[k] = v
		}
	}
	manifest := struct {
		Version                                                         int
		Repository, Environment, RoleARN, SecretVersion, WorkflowSHA256 string
		Variables                                                       map[string]string
		SecretNames                                                     []string
		Branch                                                          string
	}{1, repo, t.Environment, t.RoleARN, t.SecretVersion, digest([]byte(workflow)), vars, names, "main"}
	b, e := json.Marshal(manifest)
	return digest(b), e
}
func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

var repoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
var envPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

func Reconcile(ctx context.Context, o Options) error {
	if !repoPattern.MatchString(o.Repository) || !regexp.MustCompile(`^[0-9]{12}$`).MatchString(o.Account) || o.Region == "" || o.Runner == nil || o.Lock == nil {
		return errors.New("GitHub setup needs repository/account/region, runner and a distributed identity lock")
	}
	stdin, ok := o.Runner.(inputRunner)
	if !ok {
		return errors.New("GitHub setup runner needs protected stdin")
	}
	if o.Output == nil {
		o.Output = io.Discard
	}
	if len(o.Targets) == 0 {
		return errors.New("no GitHub setup targets selected")
	}
	seen := map[string]bool{}
	for _, t := range o.Targets {
		if !envPattern.MatchString(t.Environment) || seen[t.Environment] {
			return errors.New("invalid or duplicate GitHub environment")
		}
		seen[t.Environment] = true
		if t.SecretVersion == "" {
			return errors.New("GitHub secret version is required (nonsecret rotation identifier)")
		}
		for n, v := range t.Secrets {
			if !regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`).MatchString(n) || v == "" {
				return fmt.Errorf("required deployment secret %s missing", n)
			}
		}
	}
	c := client{ctx, o, stdin}
	if e := c.run(nil, "gh", "auth", "status", "--hostname", "github.com"); e != nil {
		return e
	}
	var identity struct{ Login string }
	if e := c.api("GET", "user", nil, &identity); e != nil {
		return e
	}
	if identity.Login == "" {
		return errors.New("cannot verify GitHub user")
	}
	var repo struct {
		FullName string `json:"full_name"`
	}
	if e := c.api("GET", "repos/"+o.Repository, nil, &repo); e != nil {
		return e
	}
	if !strings.EqualFold(repo.FullName, o.Repository) {
		return errors.New("GitHub repository mismatch")
	}
	fmt.Fprintf(o.Output, "GitHub setup as %s for %s; workflow %s; targets:", identity.Login, o.Repository, o.Workflow)
	for _, t := range o.Targets {
		fmt.Fprintf(o.Output, " %s", t.Environment)
	}
	fmt.Fprintln(o.Output)
	release, e := o.Lock()
	if e != nil {
		return e
	}
	defer release()
	var pages []struct{ Environments []struct{ Name string } }
	if e := c.pages("repos/"+o.Repository+"/environments", &pages); e != nil {
		return e
	}
	exists := map[string]bool{}
	for _, p := range pages {
		for _, e := range p.Environments {
			exists[e.Name] = true
		}
	}
	workflow, e := os.ReadFile(filepath.Join(o.Root, o.Workflow))
	if e != nil {
		return fmt.Errorf("deployment workflow must be checked out before setup: %w", e)
	}
	for _, name := range o.WorkflowFiles {
		b, err := os.ReadFile(filepath.Join(o.Root, name))
		if err != nil {
			return fmt.Errorf("referenced workflow %s is missing: %w", name, err)
		}
		workflow = append(workflow, []byte("\n"+name+"\x00")...)
		workflow = append(workflow, b...)
	}
	for _, target := range o.Targets {
		t := target
		role, e := c.ensureRole(t)
		if e != nil {
			return e
		}
		t.RoleARN = role
		t.Variables = copyMap(t.Variables)
		t.Variables["DEPLOY_ROLE_ARN"] = role
		if e := c.environment(t, exists[t.Environment], string(workflow)); e != nil {
			return fmt.Errorf("environment %s incomplete: %w", t.Environment, e)
		}
		fmt.Fprintf(o.Output, "Ready: https://github.com/%s/settings/environments (environment %s); https://github.com/%s/actions/workflows/%s\n", o.Repository, t.Environment, o.Repository, filepath.Base(o.Workflow))
	}
	return nil
}
func copyMap(m map[string]string) map[string]string {
	n := map[string]string{}
	for k, v := range m {
		n[k] = v
	}
	return n
}
func (c client) environment(t Target, exists bool, workflow string) error {
	if len(c.o.SecretStateKey) == 0 {
		c.o.RotateSecrets = true
	}
	var commit func() error
	var err error
	t, commit, err = c.delivery(t)
	if err != nil {
		return err
	}

	path := "repos/" + c.o.Repository + "/environments/" + url.PathEscape(t.Environment)
	if !exists {
		if e := c.api("PUT", path, map[string]any{"deployment_branch_policy": map[string]bool{"protected_branches": false, "custom_branch_policies": true}}, nil); e != nil {
			return e
		}
		if e := c.api("POST", path+"/deployment-branch-policies", map[string]string{"name": "main", "type": "branch"}, nil); e != nil {
			return e
		}
	}
	if e := c.protection(path); e != nil {
		return e
	}
	var vars []struct {
		Variables []struct{ Name, Value string }
	}
	if e := c.pages(path+"/variables", &vars); e != nil {
		return e
	}
	current := map[string]string{}
	for _, p := range vars {
		for _, v := range p.Variables {
			current[v.Name] = v.Value
		}
	}
	var secrets []struct{ Secrets []struct{ Name string } }
	if e := c.pages(path+"/secrets", &secrets); e != nil {
		return e
	}
	present := map[string]bool{}
	for _, p := range secrets {
		for _, v := range p.Secrets {
			present[v.Name] = true
		}
	}
	hash, e := Fingerprint(c.o.Repository, t, workflow)
	if e != nil {
		return e
	}
	same := current[Marker] == hash
	for _, name := range sortedKeys(t.Variables) {
		want := t.Variables[name]
		old, found := current[name]
		if found && old == want && !c.o.Force {
			continue
		}
		same = false
		if e := c.variable(path, name, want, found); e != nil {
			return e
		}
	}
	for _, name := range sortedKeys(t.Secrets) {
		// Changed fingerprints/rotation identifiers deliver secrets; readable inventory catches deletions.
		if present[name] && same && !c.o.RotateSecrets && !c.o.Force {
			continue
		}
		var key struct {
			Key string
			ID  string `json:"key_id"`
		}
		if e := c.api("GET", path+"/secrets/public-key", nil, &key); e != nil {
			return e
		}
		sealed, e := gh.SealSecret(key.Key, t.Secrets[name])
		if e != nil {
			return e
		}
		if e := c.api("PUT", path+"/secrets/"+name, map[string]string{"key_id": key.ID, "encrypted_value": sealed}, nil); e != nil {
			return e
		}
		same = false
	}
	// Re-read all managed values/inventory and restrictions immediately before publishing completion.
	for _, name := range sortedKeys(t.Variables) {
		var v struct{ Value string }
		if e := c.api("GET", path+"/variables/"+name, nil, &v); e != nil {
			return e
		}
		if v.Value != t.Variables[name] {
			return errors.New("managed variable changed concurrently; retry setup")
		}
	}
	for _, name := range sortedKeys(t.Secrets) {
		if e := c.api("GET", path+"/secrets/"+name, nil, nil); e != nil {
			return e
		}
	}
	if e := c.protection(path); e != nil {
		return e
	}
	if !same || c.o.Force || c.o.RotateSecrets {
		if e := c.variable(path, Marker, hash, current[Marker] != ""); e != nil {
			return e
		}
		return commit()
	}
	fmt.Fprintf(c.o.Output, "Unchanged GitHub environment %s: verified configuration and secret-name inventory; secret values are unreadable\n", t.Environment)
	return commit()
}
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func (c client) variable(path, name, value string, exists bool) error {
	method, endpoint := "POST", path+"/variables"
	if exists {
		method = "PATCH"
		endpoint += "/" + name
	}
	return c.api(method, endpoint, map[string]string{"name": name, "value": value}, nil)
}
func (c client) protection(path string) error {
	var env struct {
		Policy *struct {
			Protected, Custom    bool `json:"-"`
			ProtectedBranches    bool `json:"protected_branches"`
			CustomBranchPolicies bool `json:"custom_branch_policies"`
		} `json:"deployment_branch_policy"`
	}
	if e := c.api("GET", path, nil, &env); e != nil {
		return e
	}
	if env.Policy == nil || !env.Policy.CustomBranchPolicies || env.Policy.ProtectedBranches {
		return errors.New("configure custom main-only deployment branch restrictions in GitHub Settings > Environments; existing protections are preserved")
	}
	var pages []struct {
		Policies []struct{ Name, Type string } `json:"branch_policies"`
	}
	if e := c.pages(path+"/deployment-branch-policies", &pages); e != nil {
		return e
	}
	found := false
	for _, p := range pages {
		for _, b := range p.Policies {
			if b.Name != "main" || b.Type != "branch" {
				return errors.New("deployment environment permits branches/tags beyond main; review restrictions manually")
			}
			found = true
		}
	}
	if !found {
		return errors.New("deployment environment needs main branch permission; review restrictions manually")
	}
	return nil
}
