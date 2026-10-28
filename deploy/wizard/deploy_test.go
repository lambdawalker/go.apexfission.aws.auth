package wizard

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lambdawalker/go.apexfission.aws.auth/deploy/dns"
	model "github.com/lambdawalker/go.apexfission.aws.auth/registry"
	"github.com/lambdawalker/go.apexfission.aws.auth/runtimeconfig"
)

type cloudRunner struct {
	fakeRunner
	objects      map[string][]byte
	configs      map[string]map[string]configValue
	secret       string
	secretWrites int
	certRequests int
	pendingCerts map[string]string
}

func newCloudRunner() *cloudRunner {
	return &cloudRunner{objects: map[string][]byte{}, configs: map[string]map[string]configValue{}, pendingCerts: map[string]string{}}
}
func argument(a []string, k string) string {
	for i, v := range a {
		if v == k && i+1 < len(a) {
			return a[i+1]
		}
	}
	return ""
}
func (r *cloudRunner) RunInput(ctx context.Context, dir string, env []string, input io.Reader, name string, args ...string) ([]byte, error) {
	b, e := io.ReadAll(input)
	if e != nil {
		return nil, e
	}
	r.secret = strings.TrimSpace(string(b))
	r.secretWrites++
	return r.Run(ctx, dir, env, name, args...)
}
func (r *cloudRunner) Run(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, call)
	result := func(v any) ([]byte, error) { b, e := json.Marshal(v); return b, e }
	if name == "pulumi" {
		if strings.Contains(call, "stack ls") {
			return []byte(`[]`), nil
		}
		stack := argument(args, "--stack")
		if strings.HasPrefix(call, "pulumi config set") {
			if r.configs[stack] == nil {
				r.configs[stack] = map[string]configValue{}
			}
			v := args[3]
			secret := v == "--secret"
			if secret {
				v = "[secret]"
			}
			r.configs[stack][args[2]] = configValue{Value: v, Secret: secret}
			return []byte(`{}`), nil
		}
		if strings.HasPrefix(call, "pulumi config --json") {
			return result(r.configs[stack])
		}
		if strings.Contains(call, "stack export") {
			return []byte(`{"deployment":{"resources":[]}}`), nil
		}
		if strings.Contains(call, "stack output") {
			out := map[string]any{"deploymentIdentity": "next", "deploymentStack": stack, "deploymentAccountId": "123456789012", "deploymentRegion": "us-east-1", "apiUrl": "https://registry.execute-api.us-east-1.amazonaws.com", "indexUrl": "https://next.index.attestra.com", "runtimeTableArn": "arn:aws:dynamodb:us-east-1:123456789012:table/runtime", "runtimeTableName": "runtime", "domainTarget": "shared.amazonaws.com", "adminDomainTarget": "admin.amazonaws.com", "adminPoolId": "us-east-1_admin", "adminLoginDomain": "https://login.auth.us-east-1.amazoncognito.com"}
			if stack == "next-dev" {
				out["apiUrl"] = "https://next.api.dev.attestra.com"
				out["apiDomainTarget"] = "api.amazonaws.com"
				out["appDomainTarget"] = "d123.cloudfront.net"
				out["userPoolId"] = "us-east-1_pool"
				out["clientId"] = "clientid"
				out["runtimeDefaults"] = runtimeconfig.Default(stack)
			}
			return result(out)
		}
		return []byte(`{}`), nil
	}
	if name == "aws" && len(args) > 1 {
		switch args[0] + " " + args[1] {
		case "sts get-caller-identity":
			return []byte(`{"Account":"123456789012"}`), nil
		case "s3api get-bucket-versioning":
			return []byte(`{"Status":"Enabled"}`), nil
		case "s3api get-public-access-block":
			return []byte(`{"PublicAccessBlockConfiguration":{"BlockPublicAcls":true,"IgnorePublicAcls":true,"BlockPublicPolicy":true,"RestrictPublicBuckets":true}}`), nil
		case "sesv2 get-email-identity":
			return []byte(`{"VerifiedForSendingStatus":true}`), nil
		case "s3api put-object":
			key := argument(args, "--key")
			if argument(args, "--if-none-match") == "*" && r.objects[key] != nil {
				return nil, errors.New("PreconditionFailed")
			}
			b, e := os.ReadFile(argument(args, "--body"))
			if e != nil {
				return nil, e
			}
			r.objects[key] = b
			return []byte(`{"ETag":"etag"}`), nil
		case "s3api delete-object":
			delete(r.objects, argument(args, "--key"))
			return []byte(`{}`), nil
		case "s3api list-objects-v2":
			contents := []map[string]string{}
			for key := range r.objects {
				if strings.HasPrefix(key, argument(args, "--prefix")) {
					contents = append(contents, map[string]string{"Key": key})
				}
			}
			return result(map[string]any{"Contents": contents})
		case "s3api get-object":
			key := argument(args, "--key")
			for i, v := range args {
				if v == "--expected-bucket-owner" {
					return []byte(`{}`), os.WriteFile(args[i+2], r.objects[key], 0600)
				}
			}
		case "acm request-certificate":
			r.certRequests++
			host := argument(args, "--domain-name")
			arn := "arn:aws:acm:us-east-1:123456789012:certificate/" + host
			r.pendingCerts[arn] = host
			return result(map[string]string{"CertificateArn": arn})
		case "acm describe-certificate":
			arn := argument(args, "--certificate-arn")
			host := arn[strings.LastIndex(arn, "/")+1:]
			return result(map[string]any{"Certificate": map[string]any{"DomainName": host, "Status": "ISSUED", "CertificateArn": arn}})
		}
	}
	return []byte(`{}`), nil
}

type registryRecorder struct {
	changes     []model.Change
	failPublish bool
}

func (r *registryRecorder) Change(c model.Change) (model.Receipt, error) {
	r.changes = append(r.changes, c)
	if c.Operation == "begin" {
		return model.Receipt{Token: c.Token, Revision: 1}, nil
	}
	if c.Operation == "publish" && r.failPublish {
		return model.Receipt{}, errors.New("network down")
	}
	return model.Receipt{}, nil
}
func deployFixture(t *testing.T) (Options, *cloudRunner, *registryRecorder) {
	o, _ := fixture(t)
	r := newCloudRunner()
	reg := &registryRecorder{}
	o.Runner = r
	o.Registry = func(context.Context, string, string, string) (RegistryChanger, error) { return reg, nil }
	o.DNS = func(context.Context, []dns.Record) error { return nil }
	o.Args = []string{"--operation", "deploy", "--yes", "--account", "123456789012", "--region", "us-east-1", "--state-backend", "s3://private-state", "--sender-domain", "attestra.com", "--sender-address", "hello@attestra.com", "--android-associations", "[]"}
	for _, v := range [][2]string{{"index", "next.index.attestra.com"}, {"admin", "next.admin.attestra.com"}, {"api", "next.api.dev.attestra.com"}, {"app", "next.dev.attestra.com"}} {
		o.Args = append(o.Args, "--"+v[0]+"-certificate-arn", "arn:aws:acm:us-east-1:123456789012:certificate/"+v[1])
	}
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "test-secret-passphrase")
	return o, r, reg
}
func TestDeployOrderSecretsAndRerun(t *testing.T) {
	o, r, reg := deployFixture(t)
	if e := Run(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	calls := strings.Join(r.calls, "\n")
	if strings.Index(calls, "up --stack next-shared") > strings.Index(calls, "up --stack next-dev") {
		t.Fatal("shared must deploy first")
	}
	if r.secretWrites != 1 {
		t.Fatal("proof not created")
	}
	raw, e := base64.StdEncoding.DecodeString(r.secret)
	if e != nil || len(raw) < 32 {
		t.Fatal("weak proof key")
	}
	if strings.Contains(calls, r.secret) {
		t.Fatal("secret in arguments")
	}
	filepath.Walk(filepath.Join(o.Root, ".attestra"), func(path string, info os.FileInfo, e error) error {
		if e == nil && !info.IsDir() {
			b, _ := os.ReadFile(path)
			if strings.Contains(string(b), r.secret) || strings.Contains(string(b), "test-secret-passphrase") {
				t.Errorf("plaintext secret persisted %s", path)
			}
		}
		return nil
	})
	if len(reg.changes) != 3 || reg.changes[0].Operation != "begin" || reg.changes[1].Operation != "defaults" || reg.changes[2].Operation != "publish" {
		t.Fatal("wrong registry protocol", reg.changes)
	}
	if e = Run(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	if r.secretWrites != 1 {
		t.Fatal("proof regenerated")
	}
}
func TestPublicationRetryNeverRunsPulumi(t *testing.T) {
	o, r, reg := deployFixture(t)
	reg.failPublish = true
	if e := Run(context.Background(), o); e == nil || !strings.Contains(e.Error(), "publish") {
		t.Fatal("expected pending publication", e)
	}
	token := reg.changes[0].Token
	r.calls = nil
	reg.failPublish = false
	o.Args = append(o.Args, "--operation", "publish")
	if e := Run(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	for _, call := range r.calls {
		if strings.HasPrefix(call, "pulumi") {
			t.Fatal("publication reran cloud update", call)
		}
	}
	last := reg.changes[len(reg.changes)-1]
	if last.Token != token || last.Operation != "publish" {
		t.Fatal("receipt lost")
	}
}
func TestCertificateOwnershipReceiptReused(t *testing.T) {
	o, r, _ := deployFixture(t)
	t.Setenv("CLOUDFLARE_API_TOKEN", "test-token")
	o.Args = o.Args[:len(o.Args)-8]
	if e := Run(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	if r.certRequests != 4 {
		t.Fatal(r.certRequests)
	}
	if e := Run(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	if r.certRequests != 4 {
		t.Fatal("duplicated external certificate")
	}
}
func TestDNSConflictFailsDeployment(t *testing.T) {
	o, r, _ := deployFixture(t)
	o.DNS = func(context.Context, []dns.Record) error { return errors.New("conflicting record") }
	if e := Run(context.Background(), o); e == nil || !strings.Contains(e.Error(), "conflicting record") {
		t.Fatal(e)
	}
	if strings.Contains(strings.Join(r.calls, " "), "up --stack next-dev") {
		t.Fatal("continued after shared DNS failure")
	}
}
func TestCaptureCapacityCountsOnlyIncreases(t *testing.T) {
	need, available, e := captureCapacity(5, 1000, 110, [3]int{2, 10, 5})
	if e != nil || need != 3 || available != 10 {
		t.Fatal(need, available, e)
	}
	if _, _, e = captureCapacity(5, 100, 101, [3]int{}); e == nil {
		t.Fatal("invalid quota accepted")
	}
}
func TestProductionQuotaStopsBeforeSharedUpdate(t *testing.T) {
	o, r, _ := deployFixture(t)
	o.Args = append(o.Args, "--environment", "production", "--capture-reserved-concurrency", "5", "--capture-worker-max-concurrency", "2")
	if err := Run(context.Background(), o); err == nil {
		t.Fatal("missing production quota accepted")
	}
	for _, call := range r.calls {
		if strings.Contains(call, "pulumi up") || strings.Contains(call, "pulumi config set") {
			t.Fatal("updated before quota preflight", call)
		}
	}
}
func TestIdentityLockSurvivesCheckoutChange(t *testing.T) {
	o, r, _ := deployFixture(t)
	c := defaults()
	c.Account = "123456789012"
	c.StateBackend = "s3://private-state"
	x := execution{ctx: context.Background(), o: o, c: c, dir: o.Root}
	release, e := x.acquireLock()
	if e != nil {
		t.Fatal(e)
	}
	second := execution{ctx: context.Background(), o: Options{Root: t.TempDir(), Runner: r, Output: io.Discard}, c: c, dir: o.Root}
	if unlock, e := second.acquireLock(); e == nil {
		unlock()
		t.Fatal("second checkout acquired identity lock")
	}
	release()
	unlock, e := second.acquireLock()
	if e != nil {
		t.Fatal(e)
	}
	unlock()
}
func TestAutomaticPublicationRetrySkipsPulumi(t *testing.T) {
	o, r, reg := deployFixture(t)
	reg.failPublish = true
	if e := Run(context.Background(), o); e == nil {
		t.Fatal("expected publication failure")
	}
	r.calls = nil
	reg.failPublish = false
	if e := Run(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	for _, call := range r.calls {
		if strings.HasPrefix(call, "pulumi") {
			t.Fatal("retry ran Pulumi", call)
		}
	}
}
