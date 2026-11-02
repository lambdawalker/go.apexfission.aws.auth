package envreset

import (
	"encoding/json"
	"strings"
)

// ParseManifest validates the deployment-owned inventory, never dashboard input.
func ParseManifest(data []byte, controlTable string) (Manifest, error) {
	var m Manifest
	if json.Unmarshal(data, &m) != nil || len(m.Functions) == 0 || len(m.Tables) < 2 || m.Pool == "" || m.Bucket == "" || len(m.Queues) < 2 || controlTable == "" {
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
