package githubsetup

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fake struct {
	vars    map[string]string
	secrets map[string]bool
	writes  []string
	fail    string
}

func (f *fake) Run(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	path := args[len(args)-1]
	var value any
	switch {
	case strings.Contains(path, "/variables?"):
		rows := []map[string]string{}
		for k, v := range f.vars {
			rows = append(rows, map[string]string{"name": k, "value": v})
		}
		value = []any{map[string]any{"variables": rows}}
	case strings.Contains(path, "/secrets?"):
		rows := []map[string]string{}
		for k := range f.secrets {
			rows = append(rows, map[string]string{"name": k})
		}
		value = []any{map[string]any{"secrets": rows}}
	case strings.Contains(path, "deployment-branch-policies?"):
		value = []any{map[string]any{"branch_policies": []any{map[string]string{"name": "main", "type": "branch"}}}}
	default:
		return nil, errors.New("unexpected read " + path)
	}
	return json.Marshal(value)
}
func (f *fake) RunInput(ctx context.Context, dir string, env []string, in io.Reader, name string, args ...string) ([]byte, error) {
	method, path := args[2], args[3]
	if strings.Contains(path, f.fail) && f.fail != "" {
		return nil, errors.New("injected failure")
	}
	var body map[string]string
	b, _ := io.ReadAll(in)
	if len(b) > 0 {
		json.Unmarshal(b, &body)
	}
	if method != "GET" {
		f.writes = append(f.writes, method+" "+path)
	}
	var value any = map[string]any{}
	switch {
	case method == "GET" && strings.HasSuffix(path, "/public-key"):
		value = map[string]string{"key": base64.StdEncoding.EncodeToString(make([]byte, 32)), "key_id": "test"}
	case strings.Contains(path, "/variables"):
		if method == "GET" {
			n := path[strings.LastIndex(path, "/")+1:]
			value = map[string]string{"value": f.vars[n]}
		} else {
			f.vars[body["name"]] = body["value"]
		}
	case strings.Contains(path, "/secrets/"):
		n := path[strings.LastIndex(path, "/")+1:]
		if method == "PUT" {
			if strings.Contains(string(b), "secret-value") {
				return nil, errors.New("plaintext secret")
			}
			f.secrets[n] = true
		} else if !f.secrets[n] {
			return nil, errors.New("secret absent")
		}
	default:
		value = map[string]any{"deployment_branch_policy": map[string]bool{"custom_branch_policies": true, "protected_branches": false}}
	}
	return json.Marshal(value)
}
func TestEnvironmentReusesAndRepairsReadableDrift(t *testing.T) {
	target := Target{Environment: "next-dev", RoleARN: "arn:aws:iam::123456789012:role/test", SecretVersion: "1", Variables: map[string]string{"AWS_REGION": "us-east-2"}, Secrets: map[string]string{"TOKEN": "secret-value"}}
	hash, e := Fingerprint("owner/repo", target, "workflow")
	if e != nil {
		t.Fatal(e)
	}
	f := &fake{vars: map[string]string{"AWS_REGION": "us-east-2", Marker: hash, "UNRELATED": "preserve"}, secrets: map[string]bool{"TOKEN": true}}
	c := client{context.Background(), Options{Root: t.TempDir(), SecretStateKey: bytes.Repeat([]byte{7}, 32), Repository: "owner/repo", Runner: f, Output: io.Discard}, f}
	if e := c.environment(target, true, "workflow"); e != nil {
		t.Fatal(e)
	}
	// First delivery records protected local metadata; the next identical run is read-only.
	f.writes = nil
	if e := c.environment(target, true, "workflow"); e != nil {
		t.Fatal(e)
	}
	if len(f.writes) != 0 {
		t.Fatal("unchanged writes", f.writes)
	}
	target.Secrets["TOKEN"] = "changed-secret-value"
	if e := c.environment(target, true, "workflow"); e != nil {
		t.Fatal(e)
	}
	found := false
	for _, w := range f.writes {
		found = found || strings.Contains(w, "/secrets/TOKEN")
	}
	if !found {
		t.Fatal("changed local secret not redelivered")
	}
	files, _ := filepath.Glob(filepath.Join(c.o.Root, ".attestra", "github-delivery", "*.json"))
	if len(files) != 1 {
		t.Fatal("missing encrypted receipt")
	}
	b, _ := os.ReadFile(files[0])
	if bytes.Contains(b, []byte("changed-secret-value")) || bytes.Contains(b, []byte("PublishedVersion")) {
		t.Fatal("plaintext metadata")
	}
	f.writes = nil
	f.vars["AWS_REGION"] = "changed-manually"
	if e := c.environment(target, true, "workflow"); e != nil {
		t.Fatal(e)
	}
	if f.vars["AWS_REGION"] != "us-east-2" || f.vars["UNRELATED"] != "preserve" {
		t.Fatal(f.vars)
	}
	if len(f.writes) == 0 || !strings.HasSuffix(f.writes[len(f.writes)-1], Marker) {
		t.Fatal("marker must be last", f.writes)
	}
	f.writes = nil
	delete(f.secrets, "TOKEN")
	if e := c.environment(target, true, "workflow"); e != nil {
		t.Fatal(e)
	}
	if !f.secrets["TOKEN"] {
		t.Fatal("did not repair missing secret")
	}
}
func TestFailedReconcileDoesNotPublishMarker(t *testing.T) {
	target := Target{Environment: "next-dev", SecretVersion: "1", Variables: map[string]string{"AWS_REGION": "us-east-2"}, Secrets: map[string]string{"TOKEN": "secret-value"}}
	f := &fake{vars: map[string]string{}, secrets: map[string]bool{}, fail: "/secrets/public-key"}
	c := client{context.Background(), Options{Repository: "owner/repo", Runner: f, Output: io.Discard}, f}
	if e := c.environment(target, true, "workflow"); e == nil {
		t.Fatal("expected failure")
	}
	if _, ok := f.vars[Marker]; ok {
		t.Fatal("published incomplete marker")
	}
}
func TestFingerprintCanonicalAndSecretSafe(t *testing.T) {
	a := Target{Environment: "next-dev", SecretVersion: "1", Variables: map[string]string{"B": "2", "A": "1"}, Secrets: map[string]string{"TOKEN": "one"}}
	b := a
	b.Variables = map[string]string{"A": "1", "B": "2", Marker: "ignored"}
	b.Secrets = map[string]string{"TOKEN": "two"}
	h, _ := Fingerprint("owner/repo", a, "workflow")
	same, _ := Fingerprint("owner/repo", b, "workflow")
	if h != same {
		t.Fatal("hash depends on secret plaintext/map order/self marker")
	}
	b.SecretVersion = "2"
	different, _ := Fingerprint("owner/repo", b, "workflow")
	if different == h {
		t.Fatal("rotation ID not hashed")
	}
}
