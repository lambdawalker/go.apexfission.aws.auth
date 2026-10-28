package admin

import (
	"context"
	"github.com/lambdawalker/go.apexfission.aws.auth/authorization"
	"github.com/lambdawalker/go.apexfission.aws.auth/router"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestPageNavigationAndServerAuthorization(t *testing.T) {
	assets := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("Sales")}}
	var _ fs.FS = assets
	s := New(router.VerifierFunc(func(context.Context, string) (authorization.Principal, error) {
		return authorization.Principal{Subject: "admin"}, nil
	}), authorization.ScopeAuthorizer{}, []string{"dev"})
	if e := s.Register(Page{ID: "sales", Title: "Sales", Permission: "sales:admin", Assets: assets, Entry: "index.html"}); e != nil {
		t.Fatal(e)
	}
	if e := s.Register(Page{ID: "sales", Title: "Again", Permission: "sales:admin", Assets: assets, Entry: "index.html"}); e == nil {
		t.Fatal("duplicate page")
	}
	r := httptest.NewRequest("GET", "/admin/pages/sales/index.html?env=dev", nil)
	r.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatal(w.Code)
	}
}
