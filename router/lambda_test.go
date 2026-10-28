package router

import (
	"context"
	"github.com/aws/aws-lambda-go/events"
	"net/http"
	"testing"
)

func TestLambdaPreservesTrustedHostAndCookies(t *testing.T) {
	h := Lambda(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "admin.example" {
			t.Errorf("lost domain: %q", r.Host)
		}
		if _, e := r.Cookie("session"); e != nil {
			t.Error(e)
		}
		w.WriteHeader(204)
	}))
	in := events.APIGatewayV2HTTPRequest{RawPath: "/admin/session", Cookies: []string{"session=token"}}
	in.RequestContext.HTTP.Method = "POST"
	in.RequestContext.DomainName = "admin.example"
	out, e := h(context.Background(), in)
	if e != nil || out.StatusCode != 204 {
		t.Fatal(out, e)
	}
}
