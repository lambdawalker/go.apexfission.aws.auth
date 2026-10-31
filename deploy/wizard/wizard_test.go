package wizard

import (
	"context"
	"encoding/json"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentityAndDomains(t *testing.T) {
	for _, tc := range []struct{ prefix, identity, host string }{{"next", "next", "next.dev.attestra.com"}, {"", "final", "dev.attestra.com"}} {
		c := choices{Prefix: tc.prefix, Environment: "dev", Scope: "both", Operation: "preview", Account: "123456789012", Region: "eu-west-1", StateBackend: "s3://dedicated/state", BaseDomain: "attestra.com", CaptureReserved: -1, CaptureWorker: 2}
		if err := c.validate(); err != nil {
			t.Fatal(err)
		}
		if c.identity() != tc.identity || c.host("dev") != tc.host || c.backend() != "s3://dedicated/state/attestra/"+tc.identity {
			t.Fatalf("wrong isolation: %#v", c)
		}
	}
	for _, p := range []string{"final", "next-dev", "Next", "1next"} {
		c := defaults()
		c.Prefix = p
		if c.validate() == nil {
			t.Fatalf("accepted %q", p)
		}
	}
}
func TestBindingImmutable(t *testing.T) {
	s := saved{Bindings: map[string]binding{}}
	c := defaults()
	c.Account = "123456789012"
	c.StateBackend = "s3://private-state"
	if e := s.bind(c); e != nil {
		t.Fatal(e)
	}
	c.Region = "eu-west-1"
	if s.bind(c) == nil {
		t.Fatal("region rebound")
	}
	c.Prefix = ""
	if e := s.bind(c); e != nil {
		t.Fatal("blank prefix must be separate", e)
	}
}
func TestSharedMetadataRejected(t *testing.T) {
	c := defaults()
	c.Account = "123456789012"
	out := map[string]any{"deploymentIdentity": "other", "deploymentStack": "next-shared", "deploymentAccountId": c.Account, "deploymentRegion": c.Region}
	if validateOutputs(c, "next-shared", out) == nil {
		t.Fatal("accepted foreign output")
	}
	out["deploymentIdentity"] = "next"
	if err := validateOutputs(c, "next-shared", out); err != nil {
		t.Fatal(err)
	}
}

type fakeRunner struct {
	calls       []string
	selectError bool
	mismatch    bool
}

func (r *fakeRunner) Run(_ context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, call)
	if name == "pulumi" && len(args) > 0 && args[0] == "preview" {
		if path := argument(args, "--save-plan"); path != "" {
			if e := os.WriteFile(path, []byte(`{"version":1,"plan":{"resourcePlans":{"urn:test":{"steps":["update"]}}}}`), 0600); e != nil {
				return nil, e
			}
		}
	}
	switch {
	case strings.Contains(call, "get-caller-identity"):
		return []byte(`{"Account":"123456789012"}`), nil
	case strings.Contains(call, "get-bucket-location"):
		return []byte(`{"LocationConstraint":"us-east-1"}`), nil
	case strings.Contains(call, "get-bucket-encryption"):
		return []byte(`{"ServerSideEncryptionConfiguration":{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}}`), nil
	case strings.Contains(call, "get-bucket-versioning"):
		return []byte(`{"Status":"Enabled"}`), nil
	case strings.Contains(call, "get-public-access-block"):
		return []byte(`{"PublicAccessBlockConfiguration":{"BlockPublicAcls":true,"IgnorePublicAcls":true,"BlockPublicPolicy":true,"RestrictPublicBuckets":true}}`), nil
	case strings.Contains(call, "pulumi stack select") && r.selectError:
		return nil, os.ErrNotExist
	case strings.Contains(call, "pulumi stack export"):
		return []byte(`{"deployment":{"resources":[{"type":"pulumi:pulumi:Stack"}]}}`), nil
	case strings.Contains(call, "pulumi config --json"):
		region := "us-east-1"
		if r.mismatch {
			region = "eu-west-1"
		}
		c := defaults()
		c.Account = "123456789012"
		c.Region = "us-east-1"
		c.StateBackend = "s3://private-state"
		shared := strings.HasSuffix(argument(args, "--stack"), "-shared")
		project := "attestra-backend"
		if shared {
			project = "attestra-shared"
		}
		x := execution{c: c, project: project}
		out := map[string]any{"deploymentAccountId": c.Account, "deploymentRegion": c.Region, "runtimeTableName": "runtime", "runtimeTableArn": "arn:aws:dynamodb:us-east-1:123456789012:table/runtime"}
		values, _ := x.desiredConfig(shared, out)
		config := map[string]configValue{"aws:region": {Value: region}, "aws:allowedAccountIds": {Value: "[\"123456789012\"]"}, project + ":proofKey": {Value: "[secret]", Secret: true}}
		for key, value := range values {
			config[project+":"+key] = configValue{Value: value}
		}
		b, _ := json.Marshal(config)
		return b, nil
	case strings.Contains(call, "pulumi stack output"):
		stack := "next-dev"
		if strings.Contains(call, "next-shared") {
			stack = "next-shared"
		}
		b, _ := json.Marshal(map[string]any{"deploymentIdentity": "next", "deploymentStack": stack, "deploymentAccountId": "123456789012", "deploymentRegion": "us-east-1", "runtimeTableArn": "arn:aws:dynamodb:us-east-1:123456789012:table/runtime", "runtimeTableName": "runtime"})
		return b, nil
	}
	return []byte(`{}`), nil
}
func fixture(t *testing.T) (Options, *fakeRunner) {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"infra", "infra-index"} {
		os.Mkdir(filepath.Join(root, d), 0700)
		os.WriteFile(filepath.Join(root, d, "Pulumi.yaml"), []byte("name: attestra-"+map[string]string{"infra": "backend", "infra-index": "shared"}[d]+"\nruntime: go\n"), 0600)
	}
	r := &fakeRunner{}
	return Options{Root: root, Input: strings.NewReader(""), Output: io.Discard, Runner: r, Args: []string{"--account", "123456789012", "--region", "us-east-1", "--state-backend", "s3://private-state", "--scope", "both", "--operation", "preview"}, Build: func(context.Context, string, []core.BuildTarget) error { return nil }}, r
}
func TestPreviewReadOnlyAndScopeOrdering(t *testing.T) {
	o, r := fixture(t)
	if e := Run(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	calls := strings.Join(r.calls, "\n")
	for _, bad := range []string{"stack init", "config set", "pulumi up", "put-object", "request-certificate", "admin-setup"} {
		if strings.Contains(calls, bad) {
			t.Fatal(calls)
		}
	}
	if strings.Index(calls, "preview --stack next-shared") > strings.Index(calls, "preview --stack next-dev") {
		t.Fatal("wrong order")
	}
}
func TestPreviewDoesNotInitialize(t *testing.T) {
	o, r := fixture(t)
	r.selectError = true
	if e := Run(context.Background(), o); e == nil || !strings.Contains(e.Error(), "deploy") {
		t.Fatalf("want actionable error, got %v", e)
	}
	if strings.Contains(strings.Join(r.calls, " "), "stack init") {
		t.Fatal("preview initialized")
	}
}
func TestProviderMismatchStopsPreview(t *testing.T) {
	o, r := fixture(t)
	r.mismatch = true
	if e := Run(context.Background(), o); e == nil {
		t.Fatal("accepted region mismatch")
	}
	if strings.Contains(strings.Join(r.calls, " "), "pulumi preview") {
		t.Fatal("preview ran")
	}
}
func TestPrivateAtomicConfig(t *testing.T) {
	root := t.TempDir()
	s := saved{Version: 1, Choices: defaults(), Bindings: map[string]binding{}}
	if err := save(filepath.Join(root, ".attestra", "wizard.json"), s); err != nil {
		t.Fatal(err)
	}
	st, e := os.Stat(filepath.Join(root, ".attestra", "wizard.json"))
	if e != nil || st.Mode().Perm() != 0600 {
		t.Fatal("not private", st, e)
	}
}
func TestOverridesRemainPrefixBoundAndDistinct(t *testing.T) {
	c := defaults()
	c.Account = "123456789012"
	c.StateBackend = "s3://private-state"
	c.APIDomain = "other.api.dev.attestra.com"
	if c.validate() == nil {
		t.Fatal("old prefix override accepted")
	}
	c.APIDomain = "next.admin.attestra.com"
	if c.validate() == nil {
		t.Fatal("hostname collision accepted")
	}
}
func TestExplicitProfileDropsAmbientCredentials(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "ambient-access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "ambient-secret")
	t.Setenv("AWS_SESSION_TOKEN", "ambient-session")
	o, r := fixture(t)
	o.Args = append(o.Args, "--profile", "chosen")
	capture := &envCapture{fakeRunner: r}
	o.Runner = capture
	if e := Run(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	for _, v := range capture.env {
		if strings.Contains(v, "ambient-") {
			t.Fatal("ambient credentials survived explicit profile")
		}
	}
}

type envCapture struct {
	fakeRunner *fakeRunner
	env        []string
}

func (r *envCapture) Run(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	r.env = env
	return r.fakeRunner.Run(ctx, dir, env, name, args...)
}
func TestEndpointOverridesFailBeforeAWS(t *testing.T) {
	t.Setenv("AWS_ENDPOINT_URL_STS", "https://untrusted.example")
	o, r := fixture(t)
	if e := Run(context.Background(), o); e == nil {
		t.Fatal("custom STS endpoint accepted")
	}
	if len(r.calls) != 0 {
		t.Fatal("endpoint used before rejection")
	}
}
func TestHelpSucceedsWithoutAccount(t *testing.T) {
	if e := Run(context.Background(), Options{Root: t.TempDir(), Args: []string{"--help"}, Output: io.Discard}); e != nil {
		t.Fatal(e)
	}
}
func TestRegistryURLMustBeRegionalAPIGateway(t *testing.T) {
	c := defaults()
	c.Region = "us-east-1"
	x := execution{c: c}
	for _, u := range []string{"https://evil.example", "https://x.execute-api.eu-west-1.amazonaws.com", "https://x.execute-api.us-east-1.amazonaws.com/path", "https://user@x.execute-api.us-east-1.amazonaws.com", "https://x.execute-api.us-east-1.amazonaws.com?x=1"} {
		if _, e := x.registry(u); e == nil {
			t.Fatal("accepted untrusted registry endpoint", u)
		}
	}
}

type initializingRunner struct {
	fakeRunner
	initialized bool
}

func (r *initializingRunner) Run(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	if strings.Contains(call, "stack select") && !r.initialized {
		return nil, os.ErrNotExist
	}
	if strings.Contains(call, "stack ls") {
		return []byte(`[]`), nil
	}
	if strings.Contains(call, "stack init") {
		r.initialized = true
	}
	return r.fakeRunner.Run(ctx, dir, env, name, args...)
}
func TestOnlyExplicitDeploymentInitializesFinalProjectStack(t *testing.T) {
	o, _ := fixture(t)
	r := &initializingRunner{}
	o.Runner = r
	c := defaults()
	c.Prefix = ""
	x := execution{ctx: context.Background(), o: o, c: c}
	if e := x.selectStack(true, true); e != nil {
		t.Fatal(e)
	}
	if !r.initialized || x.project != "attestra-shared" || x.stack != "final-shared" {
		t.Fatal("wrong initial target")
	}
	calls := strings.Join(r.calls, " ")
	if !strings.Contains(calls, "stack init final-shared") {
		t.Fatal(calls)
	}
}
