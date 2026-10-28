// Package core defines static module composition without provisioning dependencies.
package core

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const Version = "0.1.0"

var IDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

type Dependency struct {
	ID    string
	Major int
}
type Descriptor struct {
	ID, Version  string
	Dependencies []Dependency
}
type Module interface{ Describe() Descriptor }
type RuntimeModule interface {
	Module
	Start(context.Context) error
}
type HealthModule interface {
	Module
	Health(context.Context) error
}

// Lifecycle operations are opt-in and do not grant authority over module resources.
type LifecycleModule interface {
	Module
	BeforeDeploy(context.Context) error
	AfterDeploy(context.Context) error
}
type BuildTarget struct {
	Name    string `json:"name"`
	Package string `json:"package"`
}
type BuildModule interface {
	Module
	BuildTargets() []BuildTarget
}
type PermissionModule interface {
	Module
	Permissions() []string
}

func Major(v string) (int, error) {
	if !versionPattern.MatchString(v) {
		return 0, fmt.Errorf("version %q must be stable major.minor.patch", v)
	}
	n, e := strconv.Atoi(strings.Split(v, ".")[0])
	return n, e
}
func Resolve(modules ...Module) ([]Module, error) {
	by := map[string]Module{}
	ids := []string{}
	for _, m := range modules {
		if m == nil {
			return nil, fmt.Errorf("nil module")
		}
		d := m.Describe()
		if !IDPattern.MatchString(d.ID) {
			return nil, fmt.Errorf("invalid module id %q", d.ID)
		}
		if _, e := Major(d.Version); e != nil {
			return nil, e
		}
		if by[d.ID] != nil {
			return nil, fmt.Errorf("duplicate module %s", d.ID)
		}
		by[d.ID] = m
		ids = append(ids, d.ID)
	}
	sort.Strings(ids)
	state := map[string]int{}
	out := []Module{}
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return fmt.Errorf("dependency cycle at %s", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		m := by[id]
		deps := append([]Dependency(nil), m.Describe().Dependencies...)
		sort.Slice(deps, func(i, j int) bool { return deps[i].ID < deps[j].ID })
		seen := map[string]bool{}
		for _, d := range deps {
			if seen[d.ID] {
				return fmt.Errorf("duplicate dependency %s", d.ID)
			}
			seen[d.ID] = true
			dep := by[d.ID]
			if dep == nil {
				return fmt.Errorf("%s requires missing module %s", id, d.ID)
			}
			major, _ := Major(dep.Describe().Version)
			if major != d.Major {
				return fmt.Errorf("%s requires %s major %d", id, d.ID, d.Major)
			}
			if e := visit(d.ID); e != nil {
				return e
			}
		}
		state[id] = 2
		out = append(out, m)
		return nil
	}
	for _, id := range ids {
		if e := visit(id); e != nil {
			return nil, e
		}
	}
	return out, nil
}
func Start(ctx context.Context, modules ...Module) error {
	ms, e := Resolve(modules...)
	if e != nil {
		return e
	}
	for _, m := range ms {
		if s, ok := m.(RuntimeModule); ok {
			if e = s.Start(ctx); e != nil {
				return fmt.Errorf("start %s: %w", m.Describe().ID, e)
			}
		}
	}
	return nil
}
