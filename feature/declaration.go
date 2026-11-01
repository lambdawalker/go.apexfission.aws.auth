// Package feature exposes side-effect-free composition and configuration contracts.
package feature

import (
	composer "github.com/lambdawalker/go.infra.composer"
	"github.com/lambdawalker/go.infra.composer/configuration"
)

const (
	Kind               = "apexfission:auth:Backend"
	RuntimeRequirement = "runtime_configuration"
)

func RuntimeContract() composer.Contract {
	return composer.Contract{ID: "auth.runtime-policy", Major: 1, Fields: map[string]composer.Field{"table_name": {Type: composer.String}, "table_arn": {Type: composer.String}}}
}
func IdentityContract() composer.Contract {
	return composer.Contract{ID: "auth.identity", Major: 1, Fields: map[string]composer.Field{"pool_id": {Type: composer.String}, "client_id": {Type: composer.String}, "issuer": {Type: composer.String}}}
}
func APIContract() composer.Contract {
	return composer.Contract{ID: "auth.api", Major: 1, Fields: map[string]composer.Field{"api_url": {Type: composer.String}, "pool_id": {Type: composer.String}, "client_id": {Type: composer.String}, "issuer": {Type: composer.String}}}
}

// Declare does not read credentials/configuration, build artifacts or register resources.
// The backend is a native deployment group; private AWS dependencies remain in Pulumi.
func Declare(id string) *composer.Feature {
	backend := id + ".backend"
	out := map[string]composer.Field{}
	for _, key := range []string{"api_url", "pool_id", "client_id", "issuer", "reset_function_name"} {
		out[key] = composer.Field{Type: composer.String}
	}
	return &composer.Feature{ID: id, Source: composer.Origin(), Requirements: []composer.Requirement{{ID: RuntimeRequirement, Contract: RuntimeContract(), Source: composer.Origin()}}, Resources: []composer.Resource{{ID: "backend", Kind: Kind, Source: composer.Origin(), Inputs: map[string]composer.Value{"runtime_table_name": composer.RequirementRef(id, RuntimeRequirement, "table_name"), "runtime_table_arn": composer.RequirementRef(id, RuntimeRequirement, "table_arn")}, Outputs: out}}, Offerings: []composer.Offering{
		{ID: "identity", Contract: IdentityContract(), Values: map[string]composer.Value{"pool_id": composer.ResourceRef(backend, "pool_id"), "client_id": composer.ResourceRef(backend, "client_id"), "issuer": composer.ResourceRef(backend, "issuer")}},
		{ID: "api", Contract: APIContract(), Values: map[string]composer.Value{"api_url": composer.ResourceRef(backend, "api_url"), "pool_id": composer.ResourceRef(backend, "pool_id"), "client_id": composer.ResourceRef(backend, "client_id"), "issuer": composer.ResourceRef(backend, "issuer")}},
	}}
}

// Configurable describes supported mechanisms, not hypothetical runtime adapters.
// Risk is independent of the dashboard category or storage choice.
type Configurable struct {
	Name, NativeKey, Risk, Reader string
	Type                          composer.Type
	Category                      configuration.Category
	Sensitivity                   configuration.Sensitivity
}

func ConfigurableProperties() []Configurable {
	return []Configurable{
		{"app_origin", "APP_ORIGIN", "identity-domain-migration", "env", composer.String, configuration.Application, configuration.Public},
		{"sender_address", "SENDER_ADDRESS", "sender-iam-rollout", "env", composer.String, configuration.Artifact, configuration.Public},
		{"diagnostic_mode", "DIAGNOSTIC_MODE", "public-error-disclosure", "env", composer.Boolean, configuration.Application, configuration.Public},
		{"capture_enabled", "CAPTURE_ENABLED", "onboarding-consent-policy", "env", composer.Boolean, configuration.Application, configuration.Public},
		{"proof_key", "SecretString", "cryptographic-key-migration", "secretsmanager-cold-start", composer.String, configuration.Artifact, configuration.Confidential},
	}
}

// ConfigurationContract describes bootstrap/delivery controls. The feature's full
// mutable onboarding schema remains in runtimeconfig and is administered by the app.
func ConfigurationContract(id, environment string) configuration.Declaration {
	d := configuration.Declaration{Environment: environment, PolicyVersion: "auth-configuration-v1"}
	for _, p := range ConfigurableProperties() {
		controls := []string{"scoped-access"}
		if p.Sensitivity == configuration.Confidential {
			controls = append(controls, "encrypted")
		}
		d.Properties = append(d.Properties, configuration.Property{Name: p.Name, Type: p.Type, Category: p.Category, Sensitivity: p.Sensitivity, Required: true, RequiredControls: controls})
		d.Consumers = append(d.Consumers, configuration.Consumer{Artifact: id + ".backend", Property: p.Name, Readers: []string{p.Reader}})
	}
	return d
}
