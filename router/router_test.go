package router

import (
	"context"
	"github.com/lambdawalker/go.apexfission.aws.auth/authorization"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultDeniedAndExplicitPublic(t *testing.T) {
	r := New(nil, nil, "dev")
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	if e := r.Register(Route{Method: "GET", Path: "/private", Handler: h}); e != nil {
		t.Fatal(e)
	}
	if e := r.Register(Route{Method: "GET", Path: "/public", Access: Public, Handler: h}); e != nil {
		t.Fatal(e)
	}
	for path, want := range map[string]int{"/private": 401, "/public": 204} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != want {
			t.Fatal(path, w.Code)
		}
	}
}
func TestRouteConflicts(t *testing.T) {
	r := New(nil, nil, "dev")
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	if e := r.Register(Route{Method: "GET", Path: "/items/{id}", Handler: h}); e != nil {
		t.Fatal(e)
	}
	if e := r.Register(Route{Method: "GET", Path: "/items/{other}", Handler: h}); e == nil {
		t.Fatal("ambiguous route accepted")
	}
}
func TestPermissionIsServerEnforced(t *testing.T) {
	v := VerifierFunc(func(context.Context, string) (authorization.Principal, error) {
		return authorization.Principal{Subject: "u"}, nil
	})
	r := New(v, authorization.ScopeAuthorizer{}, "dev")
	r.Register(Route{Method: "GET", Path: "/x", Permission: "sales:read", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })})
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}
