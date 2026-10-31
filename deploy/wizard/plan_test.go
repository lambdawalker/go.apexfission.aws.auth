package wizard

import (
	"encoding/json"
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

func TestUnwrappedPlanDoesNotDependOnCLIBuildVersion(t *testing.T) {
	for _, version := range []string{"v3.264.0", "3.264.0-dev.0", "dev", "v4.0.0", ""} {
		t.Run(version, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "plan.json")
			b, _ := json.Marshal(map[string]any{"manifest": map[string]string{"version": version}, "resourcePlans": map[string]any{"urn:pulumi:next-shared::attestra-shared::aws:iam/rolePolicy:RolePolicy::index-data": map[string]any{"steps": []string{"update"}, "state": map[string]string{"Action": "lambda:DeleteFunctionConcurrency"}}}})
			os.WriteFile(path, b, 0600)
			if got, err := planHasDestructiveOps(path); err != nil || got {
				t.Fatalf("CLI build metadata incorrectly blocked an update plan: destructive=%v err=%v", got, err)
			}
		})
	}
}
func TestUnwrappedPlanRequiresResourcePlanObject(t *testing.T) {
	for _, body := range []string{`{"manifest":{"version":"3.264.0"}}`, `{"resourcePlans":null}`, `{"resourcePlans":[]}`, `{"resourcePlans":{"urn":null}}`, `{"resourcePlans":{"urn":{"steps":["unknown-future-op"]}}}`} {
		path := filepath.Join(t.TempDir(), "plan.json")
		os.WriteFile(path, []byte(body), 0600)
		if _, err := planHasDestructiveOps(path); err == nil {
			t.Fatalf("unrecognized plan accepted: %s", body)
		}
	}
}

func TestActualPulumiCLIPlanFixtures(t *testing.T) {
	for _, tt := range []struct {
		name        string
		destructive bool
	}{{"update", false}, {"delete", true}} {
		got, err := planHasDestructiveOps(filepath.Join("testdata", "pulumi-v3.273-"+tt.name+".json"))
		if err != nil || got != tt.destructive {
			t.Fatalf("actual CLI %s plan: got %v err %v", tt.name, got, err)
		}
	}
}
