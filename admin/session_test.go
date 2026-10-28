package admin

import (
	"context"
	"github.com/lambdawalker/go.apexfission.aws.auth/authorization"
	"github.com/lambdawalker/go.apexfission.aws.auth/router"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestBrowserSessionLoadsNestedAssets(t *testing.T) {
	s := New(router.VerifierFunc(func(context.Context, string) (authorization.Principal, error) {
		return authorization.Principal{Subject: "admin"}, nil
	}), authorization.AuthorizerFunc(func(context.Context, authorization.Principal, string, string) error { return nil }), []string{"dev"})
	s.Register(Page{ID: "sales", Title: "Sales", Permission: "sales:admin", Entry: "index.html", Assets: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte(`<script src="assets/app.js"></script>`)}, "assets/app.js": &fstest.MapFile{Data: []byte("console.log('sales')")}}})
	r := httptest.NewRequest("POST", "https://admin.example/admin/session", strings.NewReader(`{"environment":"dev"}`))
	r.Header.Set("Authorization", "Bearer valid")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatalf("session %d", w.Code)
	}
	cookies := w.Result().Cookies()
	r = httptest.NewRequest("GET", "https://admin.example/admin/pages/sales/assets/app.js", nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "sales") {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestCookieSessionRejectsCrossOriginMutation(t *testing.T) {
	s := New(router.VerifierFunc(func(context.Context, string) (authorization.Principal, error) {
		return authorization.Principal{Subject: "admin"}, nil
	}), authorization.AuthorizerFunc(func(context.Context, authorization.Principal, string, string) error { return nil }), []string{"dev"})
	r := httptest.NewRequest("POST", "https://admin.example/admin/session", strings.NewReader(`{"environment":"dev"}`))
	r.Header.Set("Cookie", "__Host-apex-session=token")
	r.Header.Set("Origin", "https://attacker.example")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}
