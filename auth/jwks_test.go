package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJWKSFetchThrottleRotationAndExpiry(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	kid := "one"
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprintf(w, `{"keys":[{"kty":"RSA","kid":%q,"use":"sig","alg":"RS256","n":%q,"e":"AQAB"}]}`, kid, base64.RawURLEncoding.EncodeToString(key.N.Bytes()))
	}))
	defer srv.Close()
	j := &JWKS{endpoint: srv.URL, client: srv.Client()}
	ctx := context.Background()
	if _, e := j.Key(ctx, "one"); e != nil {
		t.Fatal(e)
	}
	if _, e := j.Key(ctx, "unknown"); e == nil || calls != 1 {
		t.Fatal("unknown kid throttle failed")
	}
	kid = "two"
	j.lastFetch = time.Now().Add(-2 * time.Minute)
	if _, e := j.Key(ctx, "two"); e != nil || calls != 2 {
		t.Fatal("rotation failed", e)
	}
	j.expires = time.Now().Add(-time.Second)
	if _, e := j.Key(ctx, "two"); e == nil {
		t.Fatal("expired cache accepted during throttle")
	}
}
