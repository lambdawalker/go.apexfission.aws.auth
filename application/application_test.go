package application

import (
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	"github.com/lambdawalker/go.apexfission.aws.auth/router"
	"net/http"
	"testing"
)

type sample struct{ path string }

func (sample) Describe() core.Descriptor { return core.Descriptor{ID: "sales", Version: "0.1.0"} }
func (s sample) Routes() []router.Route {
	return []router.Route{{Method: "GET", Path: s.path, Lambda: "sales", Permission: "sales:read", Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})}}
}
func (sample) Permissions() []string { return []string{"sales:read"} }
func TestComposition(t *testing.T) {
	a, e := Compose(sample{"/sales"})
	if e != nil || len(a.Routes) != 1 {
		t.Fatal(a, e)
	}
	if _, e = Compose(sample{"/sales"}, sample{"/other"}); e == nil {
		t.Fatal("duplicate module")
	}
}
