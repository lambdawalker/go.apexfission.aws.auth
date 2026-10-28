package deploy

import (
	"github.com/lambdawalker/go.apexfission.aws.auth/application"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	"testing"
)

type contributed struct{}

func (contributed) Describe() core.Descriptor { return core.Descriptor{ID: "custom", Version: "1.0.0"} }
func (contributed) BuildTargets() []core.BuildTarget {
	return []core.BuildTarget{{Name: "custom", Package: "./cmd/custom"}}
}
func TestEngineDiscoversLinkedBuildTargets(t *testing.T) {
	a, e := application.Compose(contributed{})
	if e != nil {
		t.Fatal(e)
	}
	engine := Engine{Manifest: Manifest{Modules: map[string]string{"custom": "1.0.0"}}}
	if e = engine.Use(a); e != nil {
		t.Fatal(e)
	}
	if len(engine.Manifest.Builds) != 1 || engine.Manifest.Builds[0].Name != "custom" {
		t.Fatal(engine.Manifest.Builds)
	}
}
