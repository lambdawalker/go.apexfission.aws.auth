package wizard

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyImportPrecedenceAndIsolation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	os.MkdirAll(filepath.Join(home, ".attestra"), 0700)
	path := filepath.Join(home, ".attestra", "legacy-wizard.json")
	os.WriteFile(path, []byte(`{"Version":1,"Environments":{"dev":{"SenderDomain":"dev.info.example.com","Region":"us-west-2","BaseDomain":"example.com","Prefix":"old","Operation":"deploy","APIDomain":"legacy.example.com"},"qa":{"SenderDomain":"qa.info.example.com"}}}`), 0600)
	c := defaults()
	c.Region = "eu-west-1"
	skipped, err := importLegacy(&c, map[string]json.RawMessage{"BaseDomain": json.RawMessage(`"local.example.com"`)}, map[string]bool{"region": true})
	if err != nil {
		t.Fatal(err)
	}
	if c.Region != "eu-west-1" || c.BaseDomain != "local.example.com" || c.SenderDomain != "dev.info.example.com" || !skipped[&c.SenderDomain] {
		t.Fatalf("wrong precedence: %+v", c)
	}
	if c.Prefix != "next" || c.Operation != "preview" || c.APIDomain != "" {
		t.Fatalf("legacy identity imported: %+v", c)
	}
	c = defaults()
	c.Environment = "qa"
	if _, err := importLegacy(&c, nil, nil); err != nil || c.SenderDomain != "qa.info.example.com" {
		t.Fatalf("wrong environment: %+v %v", c, err)
	}
	os.WriteFile(path, []byte(`{"Version":2}`), 0600)
	if _, err := importLegacy(&c, nil, nil); err == nil {
		t.Fatal("accepted incompatible export")
	}
}

func TestInteractiveLegacyImportSkipsAvailableQuestions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	os.MkdirAll(filepath.Join(home, ".attestra"), 0700)
	os.WriteFile(filepath.Join(home, ".attestra", "legacy-wizard.json"), []byte(`{"Version":1,"Environments":{"qa":{"SenderDomain":"qa.info.example.com","SenderAddress":"verify@qa.info.example.com","Region":"us-west-2"}}}`), 0600)
	var out bytes.Buffer
	_, c, _, err := parse(Options{Root: t.TempDir(), Output: &out, reader: bufio.NewReader(strings.NewReader("\nqa\n" + strings.Repeat("\n", 20)))})
	if err != nil {
		t.Fatal(err)
	}
	if c.SenderDomain != "qa.info.example.com" || c.Region != "us-west-2" {
		t.Fatalf("wrong import: %+v", c)
	}
	for _, label := range []string{"Sender domain", "Sender email", "AWS region"} {
		if strings.Contains(out.String(), label) {
			t.Fatalf("imported question asked: %s", label)
		}
	}
	if !strings.Contains(out.String(), "Existing S3 state base") {
		t.Fatal("missing value was not prompted")
	}
}

func TestInteractiveExplicitAccountStillImportsOtherLegacyAnswers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("ATTESTRA_ACCOUNT", "123456789012")
	if err := os.MkdirAll(filepath.Join(home, ".attestra"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".attestra", "legacy-wizard.json"), []byte(`{"Version":1,"Environments":{"qa":{"Account":"999999999999","Region":"us-west-2","StateBackend":"s3://legacy-state","SenderDomain":"qa.info.example.com","SenderAddress":"verify@qa.info.example.com"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	_, c, _, err := parse(Options{Root: t.TempDir(), Output: &out, reader: bufio.NewReader(strings.NewReader("backend\nqa\nnext\n" + strings.Repeat("\n", 20)))})
	if err != nil {
		t.Fatal(err)
	}
	if c.Account != "123456789012" || c.Region != "us-west-2" || c.StateBackend != "s3://legacy-state" || c.SenderDomain != "qa.info.example.com" || c.SenderAddress != "verify@qa.info.example.com" {
		t.Fatalf("wrong precedence/import %+v", c)
	}
	for _, label := range []string{"AWS account", "AWS region", "Existing S3 state base", "Sender domain", "Sender email"} {
		if strings.Contains(out.String(), label) {
			t.Fatalf("available answer was prompted: %s", label)
		}
	}
}
func TestScopeFallbackSharesOnlyCommonAnswers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	root := t.TempDir()
	c := defaults()
	c.Scope = "shared"
	c.Account = "123456789012"
	c.Region = "us-west-2"
	c.StateBackend = "s3://scope-state"
	c.Profile = "sso"
	s := saved{Version: 1}
	s.remember(c)
	if err := save(filepath.Join(root, ".attestra", "wizard.json"), s); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	_, got, _, err := parse(Options{Root: root, Output: &out, reader: bufio.NewReader(strings.NewReader("backend\ndev\nnext\n" + strings.Repeat("\n", 20)))})
	if err != nil {
		t.Fatal(err)
	}
	if got.Account != c.Account || got.StateBackend != c.StateBackend || got.Profile != c.Profile || got.Scope != "backend" {
		t.Fatalf("wrong fallback %+v", got)
	}
	for _, label := range []string{"AWS account", "AWS region", "Existing S3 state base", "AWS profile"} {
		if strings.Contains(out.String(), label) {
			t.Fatalf("common answer prompted: %s", label)
		}
	}
	if !strings.Contains(out.String(), "Sender domain") || !strings.Contains(out.String(), "Sender email") {
		t.Fatal("missing backend fields suppressed")
	}
	_, got, _, err = parse(Options{Root: root, Output: &out, Args: []string{"--scope", "backend", "--environment", "qa"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Account != "" || got.Profile != "" {
		t.Fatalf("cross-environment fallback %+v", got)
	}
}
