package sales

import (
	"context"
	"github.com/lambdawalker/go.apexfission.aws.auth/application"
	"github.com/lambdawalker/go.apexfission.aws.auth/auth"
	"github.com/lambdawalker/go.apexfission.aws.auth/authorization"
	"github.com/lambdawalker/go.apexfission.aws.auth/router"
	"net/http/httptest"
	"testing"
)

type store struct{ subject string }

func (s *store) List(_ context.Context, subject string) ([]Order, error) {
	s.subject = subject
	return []Order{{ID: "one", Amount: 42}}, nil
}
func (s *store) Put(_ context.Context, _ string, _ Order) error { return nil }
func TestConsumerWithoutFrameworkChanges(t *testing.T) {
	db := &store{}
	app, e := application.Compose(auth.Module{}, Module{Store: db})
	if e != nil {
		t.Fatal(e)
	}
	if len(app.Pages) != 1 {
		t.Fatal("missing admin section")
	}
	h, e := app.Handler(router.VerifierFunc(func(context.Context, string) (authorization.Principal, error) {
		return authorization.Principal{Subject: "user-1", Scopes: []string{"sales:read"}}, nil
	}), authorization.ScopeAuthorizer{}, "dev")
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("GET", "/sales/orders", nil)
	r.Header.Set("Authorization", "Bearer access")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || db.subject != "user-1" {
		t.Fatal(w.Code, db.subject)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/sales/orders", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}
