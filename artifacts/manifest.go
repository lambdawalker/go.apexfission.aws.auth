// Package artifacts is the source of truth for feature-owned Lambda builds.
package artifacts

const Module = "github.com/lambdawalker/go.apexfission.aws.auth"

type Lambda struct{ Name, Package string }

// Lambdas returns independent archive names and their pinned module command paths.
// Applications build these packages using their normal Go dependency graph.
func Lambdas() []Lambda {
	names := []string{"challenge", "passkeyoptions", "passkeycomplete", "auth-email-start", "auth-email-complete", "auth-passkey-start", "auth-passkey-complete", "auth-refresh", "auth-status", "auth-password-start", "auth-password-complete", "auth-password-set", "app-config", "environment-reset"}
	out := make([]Lambda, 0, len(names))
	for _, name := range names {
		command := name
		if len(name) > 5 && name[:5] == "auth-" {
			command = "signin"
		}
		out = append(out, Lambda{Name: name, Package: Module + "/cmd/" + command})
	}
	return out
}
