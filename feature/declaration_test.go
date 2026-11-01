package feature

import (
	"errors"
	"testing"

	composer "github.com/lambdawalker/go.infra.composer"
)

func TestDeclarationRequiresApplicationRuntimeBinding(t *testing.T) {
	f := Declare("authentication")
	_, err := composer.NewApplication("test", f).Plan(composer.Environment{Name: "dev"})
	var failure *composer.ValidationError
	if !errors.As(err, &failure) || failure.Diagnostics[0].Code != "IC_MISSING_PROVIDER" {
		t.Fatalf("missingruntimewasnotrejected:%v", err)
	}
	provider := &composer.Feature{ID: "application", Offerings: []composer.Offering{{ID: "runtime", Contract: RuntimeContract(), Values: map[string]composer.Value{"table_name": composer.Literal("table"), "table_arn": composer.Literal("arn")}}}}
	app := composer.NewApplication("test", f, provider)
	if err := app.Inject(f.ID, RuntimeRequirement, provider.ID, "runtime"); err != nil {
		t.Fatal(err)
	}
	p, err := app.Plan(composer.Environment{Name: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Resources()) != 1 || p.Resources()[0].Kind != Kind || len(p.Exports()) != 9 {
		t.Fatal("incomplete featuredeclaration", p)
	}
}
func TestConfigurationInventoryMatchesSupportedReaders(t *testing.T) {
	d := ConfigurationContract("authentication", "dev")
	if len(d.Properties) != 5 || len(d.Consumers) != 5 || len(d.Bindings) != 0 {
		t.Fatal(d)
	}
	for _, p := range d.Properties {
		if p.Name == "proof_key" && p.Sensitivity != "confidential" {
			t.Fatal("proofkeynotclassified")
		}
	}
	descriptions := ConfigurableProperties()
	descriptions[0].Name = "changed"
	if ConfigurableProperties()[0].Name == "changed" {
		t.Fatal("mutableinventory")
	}
}
