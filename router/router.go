// Package router shares route declarations between HTTP runtimes and infrastructure.
package router

import (
	"context"
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/authorization"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

type Access uint8

const (
	Authenticated Access = iota
	Public
)

type Route struct {
	Method, Path, Lambda, Permission string
	Access                           Access
	Admin                            bool
	Requires                         []string
	Handler                          http.Handler
	Validate                         func(*http.Request) error
}
type Module interface {
	core.Module
	Routes() []Route
}
type Verifier interface {
	Verify(context.Context, string) (authorization.Principal, error)
}
type VerifierFunc func(context.Context, string) (authorization.Principal, error)

func (f VerifierFunc) Verify(c context.Context, t string) (authorization.Principal, error) {
	return f(c, t)
}

type Router struct {
	mu          sync.RWMutex
	routes      []Route
	verifier    Verifier
	authorizer  authorization.Authorizer
	environment string
}

func New(v Verifier, a authorization.Authorizer, environment string) *Router {
	return &Router{verifier: v, authorizer: a, environment: environment}
}

var segment = regexp.MustCompile(`^(?:[A-Za-z0-9._~-]+|\{[a-zA-Z][a-zA-Z0-9_]*\})$`)

func Validate(r Route) error {
	if r.Method != "GET" && r.Method != "POST" && r.Method != "PUT" && r.Method != "PATCH" && r.Method != "DELETE" && r.Method != "OPTIONS" && r.Method != "HEAD" {
		return fmt.Errorf("unsupported method %q", r.Method)
	}
	if !strings.HasPrefix(r.Path, "/") || strings.HasSuffix(r.Path, "/") && r.Path != "/" {
		return fmt.Errorf("invalid route path %q", r.Path)
	}
	names := map[string]bool{}
	if r.Path != "/" {
		for _, s := range strings.Split(r.Path[1:], "/") {
			if !segment.MatchString(s) || s == "." || s == ".." {
				return fmt.Errorf("invalid path segment %q", s)
			}
			if strings.HasPrefix(s, "{") {
				if names[s] {
					return fmt.Errorf("duplicate path parameter")
				}
				names[s] = true
			}
		}
	}
	if r.Access != Public && r.Access != Authenticated {
		return fmt.Errorf("invalid access mode")
	}
	if r.Permission != "" && !authorization.ValidPermission(r.Permission) {
		return fmt.Errorf("invalid namespaced permission %q", r.Permission)
	}
	if r.Access == Public && (r.Permission != "" || r.Admin) {
		return fmt.Errorf("public route cannot require admin or permission")
	}
	if r.Admin && r.Permission == "" {
		return fmt.Errorf("admin route requires explicit permission")
	}
	return nil
}

// Conflicts rejects all overlapping templates, including literal/template overlap,
// so runtime and API Gateway cannot choose different authentication policies.
func Conflicts(a, b Route) bool {
	if a.Method != b.Method {
		return false
	}
	x, y := strings.Split(a.Path, "/"), strings.Split(b.Path, "/")
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] && !strings.HasPrefix(x[i], "{") && !strings.HasPrefix(y[i], "{") {
			return false
		}
	}
	return true
}
func (r *Router) Register(v Route) error {
	if e := Validate(v); e != nil {
		return e
	}
	if v.Handler == nil {
		return fmt.Errorf("runtime handler missing for %s", v.Path)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, old := range r.routes {
		if Conflicts(old, v) {
			return fmt.Errorf("conflicting route %s %s", v.Method, v.Path)
		}
	}
	r.routes = append(r.routes, v)
	return nil
}
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	routes := append([]Route(nil), r.routes...)
	r.mu.RUnlock()
	for _, v := range routes {
		if v.Method != req.Method {
			continue
		}
		parts, want := strings.Split(req.URL.Path, "/"), strings.Split(v.Path, "/")
		if len(parts) != len(want) {
			continue
		}
		match := true
		for i, s := range want {
			if strings.HasPrefix(s, "{") {
				if parts[i] == "" {
					match = false
					break
				}
				req.SetPathValue(s[1:len(s)-1], parts[i])
			} else if s != parts[i] {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if v.Access != Public {
			auth := strings.Fields(req.Header.Get("Authorization"))
			if len(auth) != 2 || auth[0] != "Bearer" || r.verifier == nil {
				http.Error(w, "unauthorized", 401)
				return
			}
			p, e := r.verifier.Verify(req.Context(), auth[1])
			if e != nil || p.Subject == "" {
				http.Error(w, "unauthorized", 401)
				return
			}
			if v.Permission != "" && (r.authorizer == nil || r.authorizer.Authorize(req.Context(), p, r.environment, v.Permission) != nil) {
				http.Error(w, "forbidden", 403)
				return
			}
			req = req.WithContext(authorization.WithPrincipal(req.Context(), p))
		}
		if v.Validate != nil && v.Validate(req) != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		v.Handler.ServeHTTP(w, req)
		return
	}
	http.NotFound(w, req)
}
