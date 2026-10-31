package wizard

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// planHasDestructiveOps reads Pulumi's versioned DeploymentPlanV1 operations,
// rather than searching property values or log messages for words like delete.
// The saved plan remains the constraint passed to up. Unknown formats fail closed.
func planHasDestructiveOps(path string) (bool, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return false, e
	}

	type plan struct {
		Manifest struct {
			Version string `json:"version"`
		} `json:"manifest"`
		Resources map[string]struct {
			Steps []string `json:"steps"`
		} `json:"resourcePlans"`
	}
	var root map[string]json.RawMessage
	if e = json.Unmarshal(b, &root); e != nil {
		return false, fmt.Errorf("invalid Pulumi plan JSON: %w", e)
	}
	var parsed plan
	if _, wrapped := root["version"]; wrapped {
		var envelope struct {
			Version int   `json:"version"`
			Plan    *plan `json:"plan"`
		}
		if e = json.Unmarshal(b, &envelope); e != nil {
			return false, e
		}
		if envelope.Version != 1 || envelope.Plan == nil {
			return false, errors.New("unsupported Pulumi plan format; refusing automatic approval")
		}
		parsed = *envelope.Plan
	} else {
		// Pulumi also writes the documented unwrapped DeploymentPlanV1 form.
		if e = json.Unmarshal(b, &parsed); e != nil {
			return false, e
		}
		if !strings.HasPrefix(parsed.Manifest.Version, "3.") {
			return false, errors.New("unsupported unwrapped Pulumi plan format; refusing automatic approval")
		}
	}
	destructive := false
	for _, r := range parsed.Resources {
		for _, op := range r.Steps {
			switch op {
			case "same", "create", "update", "read", "refresh", "import":
			case "delete", "replace", "create-replacement", "delete-replaced", "read-replacement", "discard", "discard-replaced", "remove-pending-replace", "import-replacement":
				destructive = true
			default:
				return false, fmt.Errorf("unrecognized Pulumi plan operation %q; refusing automatic approval", op)
			}
		}
	}
	return destructive, nil
}
