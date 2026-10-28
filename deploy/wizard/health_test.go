package wizard

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func healthExecution(f roundTrip) *execution {
	return &execution{ctx: context.Background(), c: defaults(), o: Options{Output: io.Discard, HTTP: &http.Client{Transport: f}}}
}
func TestHealthRejectsMissingRoute(t *testing.T) {
	x := healthExecution(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})
	if x.health(map[string]any{"apiUrl": "https://next.api.dev.attestra.com"}) == nil {
		t.Fatal("404 reported healthy")
	}
}
func TestHealthRequiresActualStatusResponse(t *testing.T) {
	x := healthExecution(func(r *http.Request) (*http.Response, error) {
		if r.Method != "POST" || r.URL.Path != "/auth/status" {
			return nil, errors.New("wrong health probe")
		}
		return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader(`{"error":"sign_in_required"}`))}, nil
	})
	if e := x.health(map[string]any{"apiUrl": "https://next.api.dev.attestra.com"}); e != nil {
		t.Fatal(e)
	}
}
func TestHealthRejectsMalformedIndexAndAssociations(t *testing.T) {
	for _, key := range []string{"indexUrl", "assetLinksUrl"} {
		x := healthExecution(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"message":"wrong"}`))}, nil
		})
		if x.health(map[string]any{key: "https://next.dev.attestra.com"}) == nil {
			t.Fatal("malformed payload reported healthy", key)
		}
	}
}
