package wizard

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	"github.com/lambdawalker/go.apexfission.aws.auth/deploy/dns"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type argsRunner struct{ args []string }

func (r *argsRunner) Run(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
	r.args = args
	return []byte(`{}`), nil
}
func TestConfigSetTerminatesOptionParsing(t *testing.T) {
	for _, v := range []string{"-1", ""} {
		r := &argsRunner{}
		x := execution{ctx: context.Background(), o: Options{Runner: r}, project: "attestra-backend", stack: "next-dev"}
		key := "captureReservedConcurrency"
		if v == "" {
			key = "deploymentPrefix"
		}
		if e := x.set(key, v); e != nil {
			t.Fatal(e)
		}
		f := flag.NewFlagSet("set", flag.ContinueOnError)
		f.SetOutput(io.Discard)
		f.String("stack", "", "")
		f.Bool("non-interactive", false, "")
		if e := f.Parse(r.args[2:]); e != nil {
			t.Fatalf("Cobra-style option parsing failed for %q: %v", v, e)
		}
		if len(f.Args()) != 2 || f.Args()[0] != "attestra-backend:"+key || f.Args()[1] != v {
			t.Fatalf("wrong positional key/value %#v", f.Args())
		}
	}
}
func TestFailedFirstVerificationDoesNotBindIdentity(t *testing.T) {
	o, r := fixture(t)
	o.Args = append(o.Args, "--account", "000000000000")
	if e := Run(context.Background(), o); e == nil {
		t.Fatal("expected account mismatch")
	}
	o.Args = o.Args[:len(o.Args)-2]
	r.calls = nil
	if e := Run(context.Background(), o); e != nil {
		t.Fatalf("corrected unused identity rejected: %v", e)
	}
}
func TestChangedPreviewIsRejectedWithoutSavingChoices(t *testing.T) {
	o, _, _ := deployFixture(t)
	if e := Run(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	before, e := os.ReadFile(filepath.Join(o.Root, ".attestra", "wizard.json"))
	if e != nil {
		t.Fatal(e)
	}
	o.Args = append(o.Args, "--operation", "preview", "--capture-enabled=true")
	if e := Run(context.Background(), o); e == nil || !strings.Contains(e.Error(), "preview") {
		t.Fatalf("requested change must be honestly rejected: %v", e)
	}
	after, _ := os.ReadFile(filepath.Join(o.Root, ".attestra", "wizard.json"))
	if string(before) != string(after) {
		t.Fatal("failed preview saved choices")
	}
}
func TestFreshCheckoutRestoresEncryptedConfigAndPreservesProof(t *testing.T) {
	o, r, _ := deployFixture(t)
	if e := Run(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	r.remote["next-dev"]["attestra-backend:environmentResetEnabled"] = configValue{Value: "true"}
	fresh, _ := fixture(t)
	original := o.Root
	o.Root = fresh.Root
	b, _ := os.ReadFile(filepath.Join(original, ".attestra", "wizard.json"))
	if e := save(filepath.Join(o.Root, ".attestra", "wizard.json"), json.RawMessage(b)); e != nil {
		t.Fatal(e)
	}
	r.configs = map[string]map[string]configValue{}
	r.calls = nil
	o.Args = append(o.Args, "--operation", "preview")
	if e := Run(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	for _, stack := range []string{"next-shared", "next-dev"} {
		content, e := os.ReadFile(filepath.Join(o.Root, map[string]string{"next-shared": "infra-index", "next-dev": "infra"}[stack], "Pulumi."+stack+".yaml"))
		if e != nil || !strings.Contains(string(content), "encryptionsalt: preserved-provider-metadata") || !strings.Contains(string(content), "secure: encrypted-ciphertext") {
			t.Fatalf("encrypted/provider metadata not restored %s: %s %v", stack, content, e)
		}
	}
	if r.secretWrites != 1 {
		t.Fatal("fresh preview generated secret")
	}
	o.Args = append(o.Args, "--operation", "deploy")
	if e := Run(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	if r.secretWrites != 1 || r.remote["next-dev"]["attestra-backend:environmentResetEnabled"].Value != "true" {
		t.Fatal("proof or optional config lost on fresh checkout deploy")
	}
}
func TestExistingEmptyStackFailsClosedWithoutAuthoritativeConfig(t *testing.T) {
	o, r, _ := deployFixture(t)
	r.stacks["next-dev"] = true
	x := execution{ctx: context.Background(), o: o, c: choices{Prefix: "next", Environment: "dev", Account: "123456789012", Region: "us-east-1"}}
	if e := x.selectStack(false, true); e == nil || !strings.Contains(e.Error(), "authoritative") {
		t.Fatal("unsafe empty-stack config regeneration", e)
	}
	if r.secretWrites != 0 {
		t.Fatal("regenerated secret")
	}
}
func TestPublicationFailureAlreadyCompletedDNSBootstrapAndExport(t *testing.T) {
	o, r, reg := deployFixture(t)
	o.Args = append(o.Args, "--admin-email", "owner@attestra.com")
	backendDNS := false
	o.DNS = func(_ context.Context, records []dns.Record) error {
		for _, v := range records {
			if v.Name == "next.api.dev.attestra.com" {
				backendDNS = true
			}
		}
		return nil
	}
	reg.failPublish = true
	if e := Run(context.Background(), o); e == nil {
		t.Fatal("expected publication failure")
	}
	path := filepath.Join(o.Root, ".attestra", "next-dev-android.properties")
	if _, e := os.Stat(path); e != nil {
		t.Fatal("export missing before publication", e)
	}
	calls := strings.Join(r.calls, "\n")
	if !backendDNS || !strings.Contains(calls, "-email owner@attestra.com -environment next-dev") {
		t.Fatal("publication preceded DNS/bootstrap")
	}
	os.Remove(path)
	r.calls = nil
	reg.failPublish = false
	if e := Run(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(path); e != nil {
		t.Fatal("recovery did not restore current checkout Android export", e)
	}
	for _, call := range r.calls {
		if strings.HasPrefix(call, "pulumi") {
			t.Fatal("recovery reran Pulumi")
		}
	}
}
func TestBackendDNSFailureResumesSameReceiptWithoutPulumi(t *testing.T) {
	o, r, reg := deployFixture(t)
	fail := true
	o.DNS = func(_ context.Context, records []dns.Record) error {
		for _, v := range records {
			if v.Name == "next.api.dev.attestra.com" && fail {
				return errors.New("DNS not ready")
			}
		}
		return nil
	}
	if e := Run(context.Background(), o); e == nil || !strings.Contains(e.Error(), "DNS not ready") {
		t.Fatal(e)
	}
	if len(reg.changes) != 1 {
		t.Fatal("advertised endpoint before DNS")
	}
	token := reg.changes[0].Token
	r.calls = nil
	fail = false
	if e := Run(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	for _, call := range r.calls {
		if strings.HasPrefix(call, "pulumi") {
			t.Fatal("DNS recovery reran Pulumi")
		}
	}
	if reg.changes[len(reg.changes)-1].Token != token {
		t.Fatal("receipt changed during DNS recovery")
	}
}
func TestPartialStackRetainsValidLocalEncryptedConfiguration(t *testing.T) {
	o, r, _ := deployFixture(t)
	if e := Run(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	delete(r.remote, "next-dev")
	path := filepath.Join(o.Root, "infra", "Pulumi.next-dev.yaml")
	content := []byte("encryptionsalt: local-provider\nconfig:\n  attestra-backend:proofKey:\n    secure: local-ciphertext\n")
	if e := os.WriteFile(path, content, 0600); e != nil {
		t.Fatal(e)
	}
	r.calls = nil
	c := defaults()
	c.Region = "us-east-1"
	c.Account = "123456789012"
	x := execution{ctx: context.Background(), o: o, c: c}
	if e := x.selectStack(false, true); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(content) || strings.Contains(strings.Join(r.calls, " "), "config refresh") {
		t.Fatal("valid partial local encrypted config overwritten")
	}
	if r.secretWrites != 1 {
		t.Fatal("partial stack proof replaced")
	}
}
func TestBootstrapFailureResumesWithoutPulumiOrPrematurePublication(t *testing.T) {
	o, r, reg := deployFixture(t)
	o.Args = append(o.Args, "--admin-email", "owner@attestra.com")
	r.adminFailures = 1
	if e := Run(context.Background(), o); e == nil || !strings.Contains(e.Error(), "bootstrap unavailable") {
		t.Fatal(e)
	}
	if len(reg.changes) != 1 {
		t.Fatal("published before bootstrap completed")
	}
	token := reg.changes[0].Token
	r.calls = nil
	if e := Run(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	calls := strings.Join(r.calls, "\n")
	if strings.Contains(calls, "pulumi") || !strings.Contains(calls, "-email owner@attestra.com -environment next-dev") {
		t.Fatal("bootstrap recovery repeated or omitted work", calls)
	}
	if reg.changes[len(reg.changes)-1].Token != token {
		t.Fatal("bootstrap recovery changed receipt")
	}
}
func TestSharedCodeOnlyChangeRemainsPreviewable(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "dist"), 0700)
	archive := filepath.Join(root, "dist", "index.zip")
	os.WriteFile(archive, []byte("old compiled code"), 0600)
	c := defaults()
	c.Account = "123456789012"
	x := execution{c: c, project: "attestra-shared", stack: "next-shared", o: Options{Root: root, SharedBuilds: []core.BuildTarget{{Name: "index", Package: "./cmd/index"}}}}
	values, e := x.desiredConfig(true, nil)
	if e != nil {
		t.Fatal(e)
	}
	existing := map[string]configValue{}
	for key, value := range values {
		existing[x.project+":"+key] = configValue{Value: value}
	}
	os.WriteFile(archive, []byte("new compiled code"), 0600)
	if e = x.checkPreviewConfig(true, nil, existing); e != nil {
		t.Fatal("export-only source hash blocked code preview", e)
	}
}
