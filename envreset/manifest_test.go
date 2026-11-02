package envreset

import (
	"encoding/json"
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
