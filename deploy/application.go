package deploy

import (
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/application"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
)

// Use binds executable contributions before executing an operation. Build declarations
// are discovered from linked modules; optional manifest copies must agree exactly.
func (e *Engine) Use(a *application.Application) error {
	if a == nil {
		return fmt.Errorf("application required")
	}
	if err := e.Manifest.ValidateModules(a.Modules...); err != nil {
		return err
	}
	if len(e.Manifest.Builds) > 0 {
		expected := map[string]string{}
		for _, b := range a.Builds {
			expected[b.Name] = b.Package
		}
		if len(e.Manifest.Builds) != len(expected) {
			return fmt.Errorf("manifest builds differ from linked application")
		}
		for _, b := range e.Manifest.Builds {
			if expected[b.Name] != b.Package {
				return fmt.Errorf("build %s differs from linked application", b.Name)
			}
		}
	}
	e.Manifest.Builds = append([]core.BuildTarget(nil), a.Builds...)
	for _, m := range a.Modules {
		if lifecycle, ok := m.(core.LifecycleModule); ok {
			e.Before = append(e.Before, Step{ID: m.Describe().ID + ":before", Run: lifecycle.BeforeDeploy})
			e.After = append(e.After, Step{ID: m.Describe().ID + ":after", Run: lifecycle.AfterDeploy})
		}
		if health, ok := m.(core.HealthModule); ok {
			e.After = append(e.After, Step{ID: m.Describe().ID + ":health", Run: health.Health})
		}
	}
	return nil
}
