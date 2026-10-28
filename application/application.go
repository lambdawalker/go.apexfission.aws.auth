// Package application collects optional module contributions before runtime or provisioning.
package application

import (
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/admin"
	"github.com/lambdawalker/go.apexfission.aws.auth/authorization"
	"github.com/lambdawalker/go.apexfission.aws.auth/config"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	"github.com/lambdawalker/go.apexfission.aws.auth/router"
	"net/http"
	"sort"
)

type Application struct {
	Modules     []core.Module
	Routes      []router.Route
	Pages       []admin.Page
	Builds      []core.BuildTarget
	Schemas     map[string]config.Schema
	Permissions []string
}

func Compose(modules ...core.Module) (*Application, error) {
	ms, e := core.Resolve(modules...)
	if e != nil {
		return nil, e
	}
	a := &Application{Modules: ms, Schemas: map[string]config.Schema{}}
	permissions := map[string]bool{}
	pages := map[string]bool{}
	builds := map[string]bool{}
	ids := map[string]bool{}
	for _, m := range ms {
		ids[m.Describe().ID] = true
	}
	for _, m := range ms {
		id := m.Describe().ID
		if p, ok := m.(core.PermissionModule); ok {
			for _, v := range p.Permissions() {
				if !authorization.ValidPermission(v) || permissions[v] {
					return nil, fmt.Errorf("invalid/duplicate permission %s", v)
				}
				permissions[v] = true
				a.Permissions = append(a.Permissions, v)
			}
		}
		if r, ok := m.(router.Module); ok {
			for _, v := range r.Routes() {
				if e := router.Validate(v); e != nil {
					return nil, e
				}
				for _, dep := range v.Requires {
					if !ids[dep] {
						return nil, fmt.Errorf("route requires missing module %s", dep)
					}
				}
				for _, old := range a.Routes {
					if router.Conflicts(old, v) {
						return nil, fmt.Errorf("duplicate/conflicting route %s %s", v.Method, v.Path)
					}
				}
				a.Routes = append(a.Routes, v)
			}
		}
		if p, ok := m.(admin.Module); ok {
			for _, v := range p.AdminPages() {
				if pages[v.ID] {
					return nil, fmt.Errorf("duplicate admin page %s", v.ID)
				}
				pages[v.ID] = true
				a.Pages = append(a.Pages, v)
			}
		}
		if b, ok := m.(core.BuildModule); ok {
			for _, v := range b.BuildTargets() {
				if !core.IDPattern.MatchString(v.Name) || builds[v.Name] || v.Package == "" {
					return nil, fmt.Errorf("invalid/duplicate build %s", v.Name)
				}
				builds[v.Name] = true
				a.Builds = append(a.Builds, v)
			}
		}
		if c, ok := m.(config.Module); ok {
			a.Schemas[id] = c.ConfigSchema()
		}
	}
	for _, r := range a.Routes {
		if r.Permission != "" && !permissions[r.Permission] {
			return nil, fmt.Errorf("undeclared permission %s", r.Permission)
		}
	}
	for _, p := range a.Pages {
		if !permissions[p.Permission] {
			return nil, fmt.Errorf("undeclared page permission %s", p.Permission)
		}
	}
	sort.Strings(a.Permissions)
	return a, nil
}
func (a *Application) Handler(v router.Verifier, z authorization.Authorizer, env string) (http.Handler, error) {
	r := router.New(v, z, env)
	for _, route := range a.Routes {
		if route.Admin {
			continue
		}
		if e := r.Register(route); e != nil {
			return nil, e
		}
	}
	return r, nil
}
func (a *Application) Admin(v router.Verifier, z authorization.Authorizer, envs []string, store config.Store) (*admin.Shell, error) {
	s := admin.New(v, z, envs)
	for _, p := range a.Pages {
		if e := s.Register(p); e != nil {
			return nil, e
		}
	}
	for _, r := range a.Routes {
		if r.Admin {
			if e := s.RegisterRoute(r); e != nil {
				return nil, e
			}
		}
	}
	s.Config = &config.Service{Store: store, Schemas: a.Schemas, Authorize: z}
	return s, nil
}
