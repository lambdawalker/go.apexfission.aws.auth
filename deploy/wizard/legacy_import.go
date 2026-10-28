package wizard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Import only portable preferences, never resource ownership or legacy secrets.
func importLegacy(c *choices, local map[string]json.RawMessage, explicit map[string]bool) (map[*string]bool, error) {
	skipped := map[*string]bool{}
	home, err := os.UserHomeDir()
	if err != nil {
		return skipped, err
	}
	path := filepath.Join(home, ".attestra", "legacy-wizard.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return skipped, nil
	}
	if err != nil {
		return skipped, err
	}
	var export struct {
		Version      int
		Environments map[string]map[string]string
	}
	if err := json.Unmarshal(data, &export); err != nil || export.Version != 1 || export.Environments == nil {
		return skipped, fmt.Errorf("invalid legacy wizard export %s; re-run export-wizard", path)
	}
	values := export.Environments[c.Environment]
	for _, field := range []struct {
		name, flag string
		value      *string
	}{
		{"Account", "account", &c.Account}, {"Region", "region", &c.Region},
		{"StateBackend", "state-backend", &c.StateBackend}, {"BaseDomain", "base-domain", &c.BaseDomain},
		{"SenderDomain", "sender-domain", &c.SenderDomain}, {"SenderAddress", "sender-address", &c.SenderAddress},
		{"Profile", "profile", &c.Profile}, {"AndroidAssociations", "android-associations", &c.AndroidAssociations},
		{"EnvironmentClass", "environment-class", &c.EnvironmentClass},
	} {
		if explicit[field.flag] {
			continue
		}
		if raw, ok := local[field.name]; ok {
			// A locally saved empty preference is intentional too.
			if json.Unmarshal(raw, field.value) != nil {
				return skipped, fmt.Errorf("invalid local choice %s", field.name)
			}
			skipped[field.value] = true
			continue
		}
		if value := values[field.name]; value != "" {
			*field.value = value
			skipped[field.value] = true
		}
	}
	return skipped, nil
}
