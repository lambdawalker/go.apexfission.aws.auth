// Package authorization separates identity verification from application grants.
package authorization

import (
	"context"
	"errors"
	"regexp"
)

var ErrDenied = errors.New("permission denied")
var permissionPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*:[a-z][a-z0-9.-]*$`)

func ValidPermission(v string) bool { return permissionPattern.MatchString(v) }

type Principal struct {
	Subject, Issuer string
	Scopes          []string
}
type Authorizer interface {
	Authorize(context.Context, Principal, string, string) error
}
type AuthorizerFunc func(context.Context, Principal, string, string) error

func (f AuthorizerFunc) Authorize(c context.Context, p Principal, env, permission string) error {
	return f(c, p, env, permission)
}

// ScopeAuthorizer requires explicit namespaced scopes. Use a durable grant lookup for admin access.
type ScopeAuthorizer struct{}

func (ScopeAuthorizer) Authorize(_ context.Context, p Principal, _, permission string) error {
	if p.Subject == "" || !ValidPermission(permission) {
		return ErrDenied
	}
	for _, s := range p.Scopes {
		if s == permission {
			return nil
		}
	}
	return ErrDenied
}

type key struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, key{}, p)
}
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(key{}).(Principal)
	return p, ok && p.Subject != ""
}
