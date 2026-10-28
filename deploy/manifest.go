// Package deploy orchestrates Go builds and Pulumi; manifests never load executable plugins.
package deploy

import (
	"encoding/json"
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	"io"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

type Environment struct {
	Account    string `json:"account"`
	Region     string `json:"region"`
	Backend    string `json:"backend"`
	Stack      string `json:"stack"`
	Directory  string `json:"directory"`
	Production bool   `json:"production,omitempty"`
}
type Manifest struct {
	Version      int                    `json:"version"`
	Application  string                 `json:"application"`
	Modules      map[string]string      `json:"modules"`
	Builds       []core.BuildTarget     `json:"builds"`
	Environments map[string]Environment `json:"environments"`
}

func Parse(r io.Reader) (Manifest, error) {
	var m Manifest
	d := json.NewDecoder(io.LimitReader(r, 1<<20))
	d.DisallowUnknownFields()
	if e := d.Decode(&m); e != nil {
		return m, e
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return m, fmt.Errorf("manifest must contain one JSON object")
	}
	return m, m.Validate()
}
func SafePath(p string) bool {
	return p != "" && !filepath.IsAbs(p) && !strings.Contains(p, "\\") && !strings.Contains(p, ":") && p != ".." && !strings.HasPrefix(filepath.Clean(p), ".."+string(filepath.Separator))
}

var accountPattern = regexp.MustCompile(`^[0-9]{12}$`)
var regionPattern = regexp.MustCompile(`^[a-z]{2}(?:-[a-z]+)+-[0-9]+$`)

func (m Manifest) Validate() error {
	if m.Version != 1 {
		return fmt.Errorf("manifest version must be 1")
	}
	if !core.IDPattern.MatchString(m.Application) {
		return fmt.Errorf("application must be a lowercase identifier")
	}
	if len(m.Modules) == 0 {
		return fmt.Errorf("at least one statically linked module required")
	}
	for id, v := range m.Modules {
		if !core.IDPattern.MatchString(id) {
			return fmt.Errorf("invalid module %q", id)
		}
		if _, e := core.Major(v); e != nil {
			return e
		}
	}
	seen := map[string]bool{}
	for _, b := range m.Builds {
		if !core.IDPattern.MatchString(b.Name) || seen[b.Name] || !strings.HasPrefix(b.Package, "./") || !SafePath(b.Package) {
			return fmt.Errorf("invalid or duplicate build target %q", b.Name)
		}
		seen[b.Name] = true
	}
	if len(m.Environments) == 0 {
		return fmt.Errorf("environments required")
	}
	for name, e := range m.Environments {
		if !core.IDPattern.MatchString(name) || !accountPattern.MatchString(e.Account) || !regionPattern.MatchString(e.Region) || (!core.IDPattern.MatchString(e.Stack) || e.Stack != name) || !SafePath(e.Directory) {
			return fmt.Errorf("environment %s requires account, region, stack and relative directory", name)
		}
		u, err := url.Parse(e.Backend)
		if err != nil || u.Scheme != "s3" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("environment %s requires an S3 Pulumi backend", name)
		}
	}
	return nil
}

// ValidateModules compares data declarations with executable modules linked by the consumer.
func (m Manifest) ValidateModules(modules ...core.Module) error {
	resolved, e := core.Resolve(modules...)
	if e != nil {
		return e
	}
	if len(resolved) != len(m.Modules) {
		return fmt.Errorf("manifest modules differ from linked application")
	}
	for _, v := range resolved {
		d := v.Describe()
		if m.Modules[d.ID] != d.Version {
			return fmt.Errorf("module %s manifest version differs from linked Go dependency", d.ID)
		}
	}
	return nil
}
