package envreset

import (
	"encoding/json"
	"strings"
)

// ParseManifest validates the deployment-owned inventory, never dashboard input.
func ParseManifest(data []byte, controlTable string) (Manifest, error) {
	var m Manifest
	if json.Unmarshal(data, &m) != nil || len(m.Functions) == 0 || len(m.Tables) == 0 || strings.TrimSpace(m.Pool) == "" || controlTable == "" {
		return Manifest{}, ErrInvalid
	}
	if m.Bucket != "" && strings.TrimSpace(m.Bucket) == "" {
		return Manifest{}, ErrInvalid
	}
	for _, items := range [][]string{m.Functions, m.Tables, m.Queues} {
		seen := map[string]bool{}
		for _, item := range items {
			if strings.TrimSpace(item) == "" || seen[item] {
				return Manifest{}, ErrInvalid
			}
			seen[item] = true
		}
	}
	for _, table := range m.Tables {
		if table == controlTable {
			return Manifest{}, ErrInvalid
		}
	}
	return m, nil
}
