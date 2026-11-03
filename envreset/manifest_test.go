package envreset

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestComposedResetManifestIncludesAdditionalFeature(t *testing.T) {
	m := Manifest{Functions: []string{"auth-worker", "email-signup", "email-resend", "email-confirm"}, Tables: []string{"auth-grants", "id-captures", "verification-proofs"}, Pool: "pool", Bucket: "bucket", Queues: []string{"work", "dead-letter"}}
	b, _ := json.Marshal(m)
	parsed, err := ParseManifest(b, "reset-control")
	if err != nil || len(parsed.Tables) != 3 {
		t.Fatal("combined reset rejected", err)
	}
	for _, items := range []*[]string{&m.Functions, &m.Tables, &m.Queues} {
		original := *items
		*items = append(append([]string{}, original...), original[0])
		b, _ := json.Marshal(m)
		if _, err := ParseManifest(b, "reset-control"); err == nil {
			t.Fatal("duplicate reset target accepted")
		}
		*items = original
	}
	m.Tables = append(m.Tables, "reset-control")
	b, _ = json.Marshal(m)
	if _, err := ParseManifest(b, "reset-control"); err == nil {
		t.Fatal("control table can be deleted during reset")
	}
}

func TestAuthenticationOnlyResetInventory(t *testing.T) {
	m := Manifest{Functions: []string{"auth-status"}, Tables: []string{"auth-grants"}, Pool: "pool"}
	data, _ := json.Marshal(m)
	parsed, err := ParseManifest(data, "control")
	if err != nil || parsed.Bucket != "" || len(parsed.Queues) != 0 {
		t.Fatal(parsed, err)
	}
	backend := AWSBackend{Manifest: parsed}
	if err := backend.Purge(context.Background()); err != nil {
		t.Fatal(err)
	}
	if done, err := backend.Clear(context.Background(), "objects"); err != nil || !done {
		t.Fatal(done, err)
	}
	for _, change := range []func(*Manifest){func(m *Manifest) { m.Functions = nil }, func(m *Manifest) { m.Tables = nil }, func(m *Manifest) { m.Tables = []string{"control"} }, func(m *Manifest) { m.Pool = "" }, func(m *Manifest) { m.Queues = []string{""} }} {
		invalid := m
		change(&invalid)
		data, _ = json.Marshal(invalid)
		if _, err := ParseManifest(data, "control"); err == nil {
			t.Fatal("invalid auth reset accepted", invalid)
		}
	}
}
func TestResetOperationValidationMatchesExistingFormat(t *testing.T) {
	for _, value := range []string{"abcdefghijklmnop", "Ab09_-Ab09_-Ab09_", strings.Repeat("a", 80)} {
		if !ValidOperation(value) {
			t.Fatal(value)
		}
	}
	for _, value := range []string{"", strings.Repeat("a", 15), strings.Repeat("a", 81), "abcdefghijklmnop/", "abcdefghijklmnop\n"} {
		if ValidOperation(value) {
			t.Fatal(value)
		}
	}
}
