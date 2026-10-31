package wizard

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	deploy "github.com/lambdawalker/go.apexfission.aws.auth/deploy"
	"github.com/lambdawalker/go.apexfission.aws.auth/deploy/build"
	"github.com/lambdawalker/go.apexfission.aws.auth/deploy/dns"
	"github.com/lambdawalker/go.apexfission.aws.auth/deploy/githubsetup"
	"github.com/muesli/cancelreader"
)

// Options supplies the consumer root and Lambda package targets. Runner and Build
// are optional seams; production uses the local AWS/Pulumi CLI and ARM64 builder.
// A custom Runner must also implement StdinRunner to set a new encrypted proofKey.
type Options struct {
	// GitHubPolicyProfile enables a known generated IAM profile; empty keeps manual setup.
	GitHubPolicyProfile         string
	SetupGitHub                 func(context.Context, githubsetup.Options) error
	Root                        string
	Input                       io.Reader
	Output                      io.Writer
	Args                        []string
	SharedBuilds, BackendBuilds []core.BuildTarget
	Runner                      deploy.Runner
	Build                       func(context.Context, string, []core.BuildTarget) error
	HTTP                        *http.Client
	// Registry overrides signed registry transport in tests or custom integrations.
	DNS      func(context.Context, []dns.Record) error
	Registry RegistryFactory
	// ConfirmPreview overrides terminal confirmation for embedding and deterministic tests.
	ConfirmPreview func(context.Context, string, bool) error
	// ApprovalReader supplies cancellable terminal input for embedding/tests.
	ApprovalReader func() (cancelreader.CancelReader, error)
	// ApprovalTicker supplies countdown ticks and cleanup for embedding/tests.
	ApprovalTicker func() (<-chan time.Time, func())
	reader         *bufio.Reader
}
type StdinRunner interface {
	RunInput(context.Context, string, []string, io.Reader, string, ...string) ([]byte, error)
}
type process struct{}

func (process) Run(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	return process{}.RunInput(ctx, dir, env, nil, name, args...)
}
func (process) RunInput(ctx context.Context, dir string, env []string, input io.Reader, name string, args ...string) ([]byte, error) {
	deadline, cancel := context.WithTimeout(ctx, 45*time.Minute)
	defer cancel()
	c := exec.CommandContext(deadline, name, args...)
	c.Dir = dir
	c.Env = env
	c.Stdin = input
	b, e := c.CombinedOutput()
	if e != nil {
		lower := strings.ToLower(string(b))
		if strings.Contains(lower, "sso session associated") || strings.Contains(lower, "error loading sso token") || strings.Contains(lower, "token has expired") || strings.Contains(lower, "expiredtoken") {
			return nil, fmt.Errorf("%s: %w", name, ErrSSOSessionExpired)
		}
		return nil, fmt.Errorf("%s failed (output suppressed; verify session and CLI diagnostics separately): %w", name, e)
	}
	return b, nil
}

type execution struct {
	generated           map[string]*githubsetup.GeneratedPolicy
	stateKMSKeys        []string
	secretMetadataKey   []byte
	ctx                 context.Context
	o                   Options
	c                   choices
	env                 []string
	dir, stack, project string
	cloudflareToken     string
	initialized         map[string]bool
	approved            bool
}

func (x *execution) run(name string, args ...string) ([]byte, error) {
	return x.o.Runner.Run(x.ctx, x.dir, x.env, name, args...)
}
func (x *execution) aws(result any, args ...string) error {
	args = append(args, "--region", x.c.Region, "--output", "json", "--no-cli-pager", "--cli-connect-timeout", "10", "--cli-read-timeout", "30")
	b, e := x.run("aws", args...)
	if e != nil {
		return e
	}
	if result != nil && json.Unmarshal(b, result) != nil {
		return errors.New("invalid AWS response")
	}
	return nil
}
func withEnv(env []string, key, value string) []string {
	out := make([]string, 0, len(env)+1)
	for _, v := range env {
		if !strings.HasPrefix(v, key+"=") {
			out = append(out, v)
		}
	}
	return append(out, key+"="+value)
}
func Run(ctx context.Context, o Options) (result error) {
	if o.Root == "" {
		o.Root = "."
	}
	root, e := filepath.Abs(o.Root)
	if e != nil {
		return e
	}
	o.Root = root
	if o.Input == nil {
		o.Input = os.Stdin
	}
	if o.Output == nil {
		o.Output = os.Stdout
	}
	o.reader = bufio.NewReader(o.Input)
	if o.Runner == nil {
		o.Runner = process{}
	}

	if o.HTTP == nil {
		o.HTTP = &http.Client{Timeout: 20 * time.Second}
	}
	client := *o.HTTP
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if client.Timeout == 0 {
		client.Timeout = 20 * time.Second
	}
	o.HTTP = &client
	s, c, yes, e := parse(o)
	if errors.Is(e, flag.ErrHelp) {
		return nil
	}
	if e != nil {
		return e
	}
	if o.Build == nil {
		o.Build = func(ctx context.Context, root string, targets []core.BuildTarget) error {
			return build.LambdasWithOptions(ctx, root, targets, build.Options{Jobs: c.BuildJobs, ForceRebuild: c.ForceRebuild, CleanCache: c.CleanCache, Output: o.Output})
		}
	}
	if c.Operation == "reconfigure" {
		s.remember(c)
		return save(filepath.Join(root, ".attestra", "wizard.json"), s)
	}
	if c.Operation == "build" {
		targets := []core.BuildTarget{}
		if c.Scope != "backend" {
			targets = append(targets, o.SharedBuilds...)
		}
		if c.Scope != "shared" {
			targets = append(targets, o.BackendBuilds...)
		}
		return o.Build(ctx, root, targets)
	}
	if e = c.validate(); e != nil {
		return e
	}
	_, alreadyBound := s.Bindings[c.identity()]
	if e = s.bind(c); e != nil {
		return e
	}
	fmt.Fprintf(o.Output, "Target %s; %s / %s; %s; shared attestra-shared/%s, backend attestra-backend/%s; operation %s, scope %s\n", c.identity(), c.Account, c.Region, c.backend(), c.stack(true), c.stack(false), c.Operation, c.Scope)
	defer func() {
		if result == nil && c.Operation != "deploy" && c.Operation != "publish" {
			s.remember(c)
			if !alreadyBound {
				delete(s.Bindings, c.identity())
			}
			result = save(filepath.Join(root, ".attestra", "wizard.json"), s)
		}
	}()
	env := os.Environ()
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		if strings.HasPrefix(parts[0], "AWS_ENDPOINT_URL") && len(parts) == 2 && parts[1] != "" {
			return fmt.Errorf("AWS endpoint override %s is unsupported; remove it before verifying a deployment target", parts[0])
		}
	}
	env = withEnv(env, "AWS_IGNORE_CONFIGURED_ENDPOINT_URLS", "true")
	for k, v := range map[string]string{"AWS_REGION": c.Region, "AWS_DEFAULT_REGION": c.Region, "PULUMI_BACKEND_URL": c.backend(), "PULUMI_SKIP_UPDATE_CHECK": "true"} {
		env = withEnv(env, k, v)
	}
	if c.Profile != "" {
		for _, key := range []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "AWS_SECURITY_TOKEN", "AWS_WEB_IDENTITY_TOKEN_FILE", "AWS_ROLE_ARN", "AWS_ROLE_SESSION_NAME"} {
			clean := env[:0]
			for _, v := range env {
				if !strings.HasPrefix(v, key+"=") {
					clean = append(clean, v)
				}
			}
			env = clean
		}
		env = withEnv(env, "AWS_PROFILE", c.Profile)
	}
	x := execution{ctx: ctx, o: o, c: c, env: env, dir: root, cloudflareToken: os.Getenv("CLOUDFLARE_API_TOKEN"), approved: yes}
	defer func() { clear(x.secretMetadataKey) }()
	terminalInput := false
	var terminalFile *os.File
	if f, ok := o.Input.(*os.File); ok && term.IsTerminal(f.Fd()) {
		terminalInput = true
		terminalFile = f
	}
	if e = x.secrets(terminalFile, terminalInput); e != nil {
		return e
	}
	if e = x.renewSSO(terminalInput); e != nil {
		return e
	}
	if c.Operation == "rotate-secrets" {
		return nil
	}
	if e = x.verifyState(); e != nil {
		return e
	}
	if c.Operation == "setup-github" || c.Operation == "deploy" {
		if c.Operation == "setup-github" {
			c.GitHubAsked = true
			c.GitHubSetup = true
		}
		if terminalInput && len(o.Args) == 0 && !c.GitHubAsked {
			fmt.Fprint(o.Output, "Set up deployment from GitHub? [Y/n]: ")
			line, err := o.reader.ReadString('\n')
			if err != nil {
				return errors.New("GitHub setup selection cancelled")
			}
			c.GitHubSetup = strings.TrimSpace(line) == "" || strings.EqualFold(strings.TrimSpace(line), "y") || strings.EqualFold(strings.TrimSpace(line), "yes")
			c.GitHubAsked = true
		}
		if c.GitHubSetup {
			if !terminalInput && !yes {
				return errors.New("GitHub setup requires explicit --yes for unattended changes")
			}
			x.c = c
			if err := x.prepareGitHubAuthorization(terminalInput); err != nil {
				return err
			}
			if x.c.GitHubSetup {
				if err := x.setupGitHub(terminalInput); err != nil {
					return err
				}
			}
			c = x.c
		}
		if c.Operation == "setup-github" {
			s.remember(c)
			return save(filepath.Join(root, ".attestra", "wizard.json"), s)
		}
	}
	if c.Operation == "deploy" {

		if c.Scope != "shared" {
			if e = x.sender(); e != nil {
				return e
			}
			if ((c.Environment == "prod" || c.Environment == "production") || c.EnvironmentClass == "production") && c.CaptureReserved < 1 {
				return errors.New("production requires explicit capture reservation/worker capacity before deployment")
			}
		}
		if c.Route53ZoneID == "" && x.cloudflareToken == "" {
			if c.Scope != "backend" && (c.IndexCertificateARN == "" || c.AdminCertificateARN == "") || c.Scope != "shared" && (c.APICertificateARN == "" || (c.AndroidAssociations != "" && c.AppCertificateARN == "")) {
				return errors.New("DNS/TLS prerequisite: set CLOUDFLARE_API_TOKEN, supply preissued certificate ARNs, or choose --route53-zone-id before deployment")
			}
		}
		s.remember(c)
		if e = save(filepath.Join(root, ".attestra", "wizard.json"), s); e != nil {
			return e
		}
		release, e := x.acquireLock()
		if e != nil {
			return e
		}
		defer release()
		if c.Scope != "shared" {
			pending, e := x.loadPublication()
			if e != nil {
				return e
			}
			if pending != nil && (pending.Stage == "postapply" || pending.Stage == "ready" || pending.Stage == "defaults") {
				fmt.Fprintln(o.Output, "Resuming saved deployment completion and publication without rerunning infrastructure.")
				return x.completeApplied(pending)
			}
		}
	}
	if c.Operation == "publish" {
		s.remember(c)
		if e = save(filepath.Join(root, ".attestra", "wizard.json"), s); e != nil {
			return e
		}
		return x.resumePublication()
	}
	if c.Operation == "deploy" || c.Operation == "preview" {
		targets := []core.BuildTarget{}
		if c.Scope != "backend" {
			targets = append(targets, o.SharedBuilds...)
		}
		if c.Scope != "shared" {
			targets = append(targets, o.BackendBuilds...)
		}
		if e = o.Build(ctx, root, targets); e != nil {
			return e
		}
	}
	if c.Operation == "deploy" && c.Scope != "shared" {
		if e = x.preflightBackend(); e != nil {
			return e
		}
	}
	var shared map[string]any
	if c.Scope != "backend" {
		shared, e = x.stackRun(true, nil)
		if e != nil {
			return e
		}
	}
	if c.Scope != "shared" {
		if shared == nil {
			if e = x.selectStack(true, false); e != nil {
				return e
			}
			shared, e = x.outputs()
			if e != nil {
				return e
			}
		}
		if e = validateShared(c, shared); e != nil {
			return e
		}
		if _, e = x.stackRun(false, shared); e != nil {
			return e
		}
	}
	return nil
}
func (x *execution) locateStack(shared bool) error {
	x.project = "attestra-backend"
	d := "infra"
	if shared {
		x.project = "attestra-shared"
		d = "infra-index"
	}
	x.stack = x.c.stack(shared)
	x.dir = filepath.Join(x.o.Root, d)
	real, e := filepath.EvalSymlinks(x.dir)
	if e != nil {
		return e
	}
	rel, e := filepath.Rel(x.o.Root, real)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return errors.New("infrastructure directory escapes consumer root")
	}
	project, e := os.ReadFile(filepath.Join(x.dir, "Pulumi.yaml"))
	if e != nil {
		return e
	}
	found := false
	for _, line := range strings.Split(string(project), "\n") {
		if strings.TrimSpace(line) == "name: "+x.project {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("%s/Pulumi.yaml must declare name: %s", d, x.project)
	}
	return nil
}
func (x *execution) selectStack(shared, initialize bool) error {
	if e := x.locateStack(shared); e != nil {
		return e
	}
	if _, e := x.run("pulumi", "stack", "select", x.stack, "--non-interactive"); e != nil {
		if !initialize {
			return fmt.Errorf("existing stack %s unavailable; run explicit --operation deploy --yes to initialize (preview/status never initialize): %w", x.stack, e)
		}
		b, e := x.run("pulumi", "stack", "ls", "--json")
		if e != nil {
			return e
		}
		var stacks []struct{ Name string }
		if json.Unmarshal(b, &stacks) != nil {
			return errors.New("cannot safely check existing stacks")
		}
		for _, s := range stacks {
			if s.Name == x.stack || strings.HasSuffix(s.Name, "/"+x.stack) {
				return errors.New("stack exists but cannot be selected; refusing initialization")
			}
		}
		if _, e = x.run("pulumi", "stack", "init", x.stack, "--non-interactive"); e != nil {
			return e
		}
		if x.initialized == nil {
			x.initialized = map[string]bool{}
		}
		x.initialized[x.stack] = true
	}
	if x.initialized[x.stack] {
		return nil
	}
	return x.restoreConfig()
}

type configValue struct {
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

func (x *execution) config() (map[string]configValue, error) {
	b, e := x.run("pulumi", "config", "--json", "--stack", x.stack)
	if e != nil {
		return nil, e
	}
	var c map[string]configValue
	if json.Unmarshal(b, &c) != nil {
		return nil, errors.New("invalid Pulumi config")
	}
	return c, nil
}
func (x *execution) checkPins(existing map[string]configValue, allowMissing bool) error {
	for k := range existing {
		if strings.HasPrefix(k, "aws:") && k != "aws:region" && k != "aws:allowedAccountIds" {
			return fmt.Errorf("unreviewed AWS provider override %s", k)
		}
	}
	var ids []string
	region := existing["aws:region"].Value
	accounts := existing["aws:allowedAccountIds"].Value
	if region != "" && region != x.c.Region || accounts != "" && (json.Unmarshal([]byte(accounts), &ids) != nil || len(ids) != 1 || ids[0] != x.c.Account) {
		return errors.New("existing stack AWS provider account/region mismatch")
	}
	for k, want := range map[string]string{"expectedAccountId": x.c.Account, "expectedRegion": x.c.Region, "deploymentPrefix": x.c.Prefix} {
		if old, ok := existing[x.project+":"+k]; ok && old.Value != want {
			return fmt.Errorf("existing stack %s mismatch", k)
		}
	}
	if !allowMissing && (region == "" || accounts == "") {
		return errors.New("existing stack must pin aws:region and aws:allowedAccountIds; use explicit deploy to configure")
	}
	return nil
}
func (x *execution) pin(existing map[string]configValue, write bool) error {
	if e := x.checkPins(existing, write); e != nil {
		return e
	}
	if write {
		if e := x.set("aws:region", x.c.Region); e != nil {
			return e
		}
		return x.set("aws:allowedAccountIds", "[\""+x.c.Account+"\"]")
	}
	return nil
}
func (x *execution) set(k, v string) error {
	if !strings.Contains(k, ":") {
		k = x.project + ":" + k
	}
	_, e := x.run("pulumi", "config", "set", "--stack", x.stack, "--non-interactive", "--", k, v)
	return e
}
func (x *execution) outputs() (map[string]any, error) {
	b, e := x.run("pulumi", "stack", "output", "--json", "--stack", x.stack)
	if e != nil {
		return nil, e
	}
	var out map[string]any
	if json.Unmarshal(b, &out) != nil {
		return nil, errors.New("invalid stack outputs")
	}
	if e = validateOutputs(x.c, x.stack, out); e != nil {
		return nil, e
	}
	return out, nil
}
func str(m map[string]any, k string) string { s, _ := m[k].(string); return s }
func validateOutputs(c choices, stack string, out map[string]any) error {
	for k, want := range map[string]string{"deploymentIdentity": c.identity(), "deploymentStack": stack, "deploymentAccountId": c.Account, "deploymentRegion": c.Region} {
		if str(out, k) != want {
			return fmt.Errorf("shared/backend output %s mismatch for %s", k, stack)
		}
	}
	return nil
}
func validateShared(c choices, out map[string]any) error {
	if e := validateOutputs(c, c.stack(true), out); e != nil {
		return e
	}
	name := str(out, "runtimeTableName")
	if name == "" || str(out, "runtimeTableArn") != "arn:aws:dynamodb:"+c.Region+":"+c.Account+":table/"+name {
		return errors.New("shared runtime table ARN/name does not match verified target")
	}
	return nil
}
func (x *execution) stackRun(shared bool, sharedOut map[string]any) (map[string]any, error) {
	write := x.c.Operation == "deploy"
	if e := x.selectStack(shared, write); e != nil {
		return nil, e
	}
	existing, e := x.config()
	if e != nil {
		return nil, e
	}
	if e = x.pin(existing, write); e != nil {
		return nil, e
	}
	if !write {
		if x.c.Operation == "preview" {
			if e := x.checkPreviewConfig(shared, sharedOut, existing); e != nil {
				return nil, e
			}
			b, e := x.run("pulumi", "preview", "--refresh", "--stack", x.stack, "--non-interactive", "--diff")
			if e != nil {
				return nil, e
			}
			fmt.Fprint(x.o.Output, string(b))
		}
		out, e := x.outputs()
		if e != nil {
			return nil, e
		}
		if x.c.Operation == "health" {
			if !shared && existing["attestra-backend:androidAssociations"].Value != "" && str(out, "assetLinksUrl") == "" {
				return nil, errors.New("configured Android association hosting is missing")
			}
			e = x.health(out)
		} else if x.c.Operation == "status" {
			b, _ := json.MarshalIndent(out, "", "  ")
			fmt.Fprintln(x.o.Output, string(b))
		}
		return out, e
	}
	if !shared {
		if e = x.sender(); e != nil {
			return nil, e
		}
		if e = x.capacity(existing); e != nil {
			return nil, e
		}
	}
	if e = x.configure(shared, sharedOut, existing); e != nil {
		return nil, e
	}
	if e = x.certificates(shared, existing); e != nil {
		return nil, e
	}
	var receipt *publication
	if !shared {
		receipt, e = x.beginPublication(sharedOut)
		if e != nil {
			return nil, e
		}
	}
	planDir, e := os.MkdirTemp("", "attestra-reviewed-*")
	if e != nil {
		return nil, e
	}
	planPath := filepath.Join(planDir, "plan.json")
	defer os.RemoveAll(planDir)
	b, e := x.run("pulumi", "preview", "--refresh", "--stack", x.stack, "--non-interactive", "--diff", "--save-plan", planPath)
	if e != nil {
		return nil, e
	}
	fmt.Fprint(x.o.Output, string(b))
	if e = x.confirmPreview(string(b)); e != nil {
		return nil, e
	}
	b, e = x.run("pulumi", "up", "--stack", x.stack, "--non-interactive", "--yes", "--plan", planPath)
	if e != nil {
		return nil, e
	}
	fmt.Fprint(x.o.Output, string(b))
	out, e := x.outputs()
	if e != nil {
		return nil, e
	}
	if !shared {
		receipt.JournalVersion = 1
		receipt.Desired = x.c
		receipt.Outputs = out
		receipt.Shared = sharedOut
		receipt.Stage = "postapply"
		if e = x.storePublication(receipt); e != nil {
			return nil, e
		}
		return out, x.completeApplied(receipt)
	}
	if e = x.hostDNS(true, out); e != nil {
		return nil, e
	}
	_, e = x.o.Runner.Run(x.ctx, x.o.Root, x.env, "go", "run", "./cmd/admin-setup", "-auth-only", "-pool", str(out, "adminPoolId"), "-login", str(out, "adminLoginDomain"))
	if e != nil {
		return nil, e
	}

	return out, nil
}
func (x *execution) desiredConfig(shared bool, out map[string]any) (map[string]string, error) {
	index, admin, api, origin := x.c.domains()
	values := map[string]string{"deploymentPrefix": x.c.Prefix, "expectedAccountId": x.c.Account, "expectedRegion": x.c.Region}
	if x.c.GitHubAutomatic {
		prefix, boundary := x.c.GitHubBackendPrefix, x.c.GitHubBackendBoundary
		if shared {
			prefix, boundary = x.c.GitHubSharedPrefix, x.c.GitHubSharedBoundary
		}
		if prefix == "" || boundary == "" {
			return nil, errors.New("automatic runtime permissions are not approved for this target; run --operation setup-github first")
		}
		values["deploymentRolePrefix"], values["deploymentBoundaryArn"] = prefix, boundary
	}
	if shared {
		values["domain"] = index
		values["adminDomain"] = admin
		hash := sha256.New()
		targets := append([]core.BuildTarget{}, x.o.SharedBuilds...)
		sort.Slice(targets, func(i, j int) bool { return targets[i].Name < targets[j].Name })
		for _, target := range targets {
			b, e := os.ReadFile(filepath.Join(x.o.Root, "dist", target.Name+".zip"))
			if e != nil {
				return nil, e
			}
			hash.Write([]byte(target.Name))
			hash.Write(b)
		}
		values["sourceHash"] = hex.EncodeToString(hash.Sum(nil))
	} else {
		values["environment"] = x.c.Environment
		values["sharedStack"] = x.c.stack(true)
		values["sharedAccountId"] = str(out, "deploymentAccountId")
		values["sharedRegion"] = str(out, "deploymentRegion")
		values["runtimeTableName"] = str(out, "runtimeTableName")
		values["runtimeTableArn"] = str(out, "runtimeTableArn")
		values["apiDomain"] = api
		values["appOrigin"] = origin
		values["indexUrl"] = "https://" + index + "/v1/environments"
		values["senderDomain"] = x.c.SenderDomain
		values["senderAddress"] = x.c.SenderAddress
		values["senderIdentityArn"] = "arn:aws:ses:" + x.c.Region + ":" + x.c.Account + ":identity/" + x.c.SenderDomain
		values["captureEnabled"] = fmt.Sprint(x.c.CaptureEnabled)
		values["captureReservedConcurrency"] = fmt.Sprint(x.c.CaptureReserved)
		values["captureWorkerMaxConcurrency"] = fmt.Sprint(x.c.CaptureWorker)
		if x.c.EnvironmentClass != "" {
			values["environmentClass"] = x.c.EnvironmentClass
		}
		if x.c.AndroidAssociations != "" {
			values["androidAssociations"] = x.c.AndroidAssociations
		}
	}
	if x.c.Route53ZoneID != "" {
		values["route53ZoneId"] = x.c.Route53ZoneID
	}
	return values, nil
}
func (x *execution) configure(shared bool, out map[string]any, existing map[string]configValue) error {
	values, e := x.desiredConfig(shared, out)
	if e != nil {
		return e
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if e := x.set(k, values[k]); e != nil {
			return e
		}
	}
	if !shared {
		proof, ok := existing[x.project+":proofKey"]
		if ok {
			if !proof.Secret {
				return errors.New("existing proofKey is not encrypted; secure it explicitly before proceeding")
			}
		} else {
			r, ok := x.o.Runner.(StdinRunner)
			if !ok {
				return errors.New("custom runner must implement StdinRunner to securely configure proofKey")
			}
			key := make([]byte, 32)
			if _, e := rand.Read(key); e != nil {
				return e
			}
			encoded := base64.StdEncoding.EncodeToString(key)
			clear(key)
			_, e := r.RunInput(x.ctx, x.dir, x.env, bytes.NewBufferString(encoded+"\n"), "pulumi", "config", "set", "--secret", "--stack", x.stack, "--non-interactive", "--", x.project+":proofKey")
			if e != nil {
				return e
			}
		}
	}
	return nil
}

// InteractiveRunner streams public SSO authorization instructions and must
// honour context cancellation.
// ErrSSOSessionExpired lets custom runners request interactive SSO renewal.
var ErrSSOSessionExpired = errors.New("AWS session credentials require renewal")

type InteractiveRunner interface {
	RunInteractive(context.Context, string, []string, io.Reader, io.Writer, string, ...string) error
}

func (process) RunInteractive(ctx context.Context, dir string, env []string, input io.Reader, output io.Writer, name string, args ...string) error {
	deadline, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(deadline, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = input
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("interactive %s failed: %w", name, err)
	}
	return nil
}
func (x *execution) renewSSO(interactive bool) error {
	if x.c.Profile == "" || !interactive || os.Getenv("CI") != "" {
		return nil
	}
	initial := x.aws(nil, "sts", "get-caller-identity")
	if initial == nil {
		return nil
	}
	if !errors.Is(initial, ErrSSOSessionExpired) {
		return initial
	}
	session, se := x.run("aws", "configure", "get", "sso_session", "--profile", x.c.Profile)
	start, ue := x.run("aws", "configure", "get", "sso_start_url", "--profile", x.c.Profile)
	if (se != nil || strings.TrimSpace(string(session)) == "") && (ue != nil || strings.TrimSpace(string(start)) == "") {
		return initial
	}
	runner, ok := x.o.Runner.(InteractiveRunner)
	if !ok {
		return errors.New("SSO renewal requires an InteractiveRunner to display authorization instructions")
	}
	if e := runner.RunInteractive(x.ctx, x.dir, x.env, x.o.Input, x.o.Output, "aws", "sso", "login", "--profile", x.c.Profile, "--no-browser", "--use-device-code"); e != nil {
		return e
	}
	var caller struct{ Account string }
	if e := x.aws(&caller, "sts", "get-caller-identity"); e != nil {
		return e
	}
	if caller.Account != x.c.Account {
		return errors.New("AWS STS account mismatch after SSO login")
	}
	return nil
}
