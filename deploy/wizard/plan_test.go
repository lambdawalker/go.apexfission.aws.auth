package wizard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewedPlanOperationsDriveApproval(t *testing.T) {
	for _, tt := range []struct {
		steps string
		want  bool
	}{{`["update"]`, false}, {`["same","create","read","refresh","import"]`, false}, {`["delete"]`, true}, {`["create-replacement","replace","delete-replaced"]`, true}, {`["discard-replaced"]`, true}} {
		path := filepath.Join(t.TempDir(), "plan.json")
		body := `{"version":1,"plan":{"resourcePlans":{"urn:pulumi:dev::app::aws:iam/rolePolicy:RolePolicy::logging":{"steps":` + tt.steps + `,"state":{"Action":"lambda:DeleteFunctionConcurrency","Description":"replace and destroy are ordinary property values"}}}}}`
		if e := os.WriteFile(path, []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
		got, e := planHasDestructiveOps(path)
		if e != nil || got != tt.want {
			t.Fatalf("%s: %v %v", tt.steps, got, e)
		}
	}
}
func TestReviewedPlanRejectsUnknownFormatAndOperations(t *testing.T) {
	for _, body := range []string{`{}`, `{"version":2,"plan":{}}`, `{"version":1,"plan":{"resourcePlans":{"urn":{"steps":["new-unknown-operation"]}}}}`, `not json`} {
		path := filepath.Join(t.TempDir(), "plan.json")
		os.WriteFile(path, []byte(body), 0600)
		if _, e := planHasDestructiveOps(path); e == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	path := filepath.Join(t.TempDir(), "empty.json")
	os.WriteFile(path, []byte(`{"version":1,"plan":{}}`), 0600)
	if got, e := planHasDestructiveOps(path); e != nil || got {
		t.Fatal(got, e)
	}
	if _, e := planHasDestructiveOps(strings.Repeat("missing", 2)); e == nil {
		t.Fatal("missing plan accepted")
	}
}

func TestDocumentedUnwrappedPulumiPlan(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.json")
	os.WriteFile(path, []byte(`{"manifest":{"version":"3.264.0"},"resourcePlans":{"urn":{"steps":["update"],"state":{"policy":"delete"}}}}`), 0600)
	if got, e := planHasDestructiveOps(path); e != nil || got {
		t.Fatal(got, e)
	}
}
