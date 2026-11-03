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
	if len(p.Resources()) != 1 || p.Resources()[0].Kind != Kind || len(p.Exports()) != 27 {
		t.Fatal("incomplete featuredeclaration", p)
	}
}
func TestConfigurationInventoryMatchesSupportedReaders(t *testing.T) {
	d := ConfigurationContract("authentication", "dev")
	if len(d.Properties) != 3 || len(d.Consumers) != 3 || len(d.Bindings) != 0 {
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

func TestCaptureBindingIsExplicitAndReadOnly(t *testing.T) {
	standalone := Declare("auth")
	if len(standalone.Requirements) != 1 {
		t.Fatal("standalone unexpectedly needs capture", standalone.Requirements)
	}
	if _, exists := standalone.Resources[0].Inputs["capture_table_name"]; exists {
		t.Fatal("standalone has capture input")
	}
	f := DeclareWithCapture("auth")
	if len(f.Requirements) != 2 || f.Requirements[1].ID != CaptureFactsRequirement || f.Requirements[1].Contract.ID != "capture.facts" {
		t.Fatal(f.Requirements)
	}
	provider := &composer.Feature{ID: "application", Offerings: []composer.Offering{{ID: "runtime", Contract: RuntimeContract(), Values: map[string]composer.Value{"table_name": composer.Literal("runtime"), "table_arn": composer.Literal("arn:runtime")}}, {ID: "capture", Contract: CaptureFactsContract(), Values: map[string]composer.Value{"table_name": composer.Literal("external-capture"), "table_arn": composer.Literal("arn:external-capture")}}}}
	app := composer.NewApplication("test", f, provider)
	if err := app.Inject(f.ID, RuntimeRequirement, provider.ID, "runtime"); err != nil {
		t.Fatal(err)
	}
	missing := composer.NewApplication("missing", f)
	if _, err := missing.Plan(composer.Environment{Name: "dev"}); err == nil {
		t.Fatal("missing capture facts provider accepted")
	}
	if err := app.Inject(f.ID, CaptureFactsRequirement, provider.ID, "capture"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Plan(composer.Environment{Name: "dev"}); err != nil {
		t.Fatal(err)
	}
	c := CaptureIntegrationContract()
	if c.ID != "identity.card-capture" || c.Major != 1 || len(c.Fields) != 8 {
		t.Fatal(c)
	}
	for _, k := range []string{"api_id", "api_execution_arn", "pool_id", "client_id", "issuer", "region", "reset_table_name", "reset_table_arn"} {
		if c.Fields[k].Type != composer.String {
			t.Fatal("missing integration field", k)
		}
	}
}
