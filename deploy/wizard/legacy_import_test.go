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
