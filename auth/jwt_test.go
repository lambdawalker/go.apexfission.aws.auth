package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestJWTTrustBoundaries(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := Verifier{Issuer: "https://issuer.example/pool", ClientID: "client", Keys: StaticKeys{"k": &key.PublicKey}, Now: func() time.Time { return time.Unix(1000, 0) }}
	sign := func(c map[string]any) string {
		h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"k"}`))
		b, _ := json.Marshal(c)
		p := h + "." + base64.RawURLEncoding.EncodeToString(b)
		sum := sha256.Sum256([]byte(p))
		s, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
		return p + "." + base64.RawURLEncoding.EncodeToString(s)
	}
	claims := func() map[string]any {
		return map[string]any{"iss": v.Issuer, "client_id": "client", "sub": "user", "exp": 1100, "iat": 900, "token_use": "access", "scope": "sales:read"}
	}
	if p, e := v.Verify(context.Background(), sign(claims())); e != nil || p.Subject != "user" {
		t.Fatal(p, e)
	}
	for field, value := range map[string]any{"iss": "https://evil.example", "client_id": "other", "exp": 999, "token_use": "id", "sub": "", "nbf": 1100, "iat": 1100} {
		c := claims()
		c[field] = value
		if _, e := v.Verify(context.Background(), sign(c)); e == nil {
			t.Fatal("accepted", field)
		}
	}
	if _, e := v.Verify(context.Background(), sign(claims())+"bad"); e == nil {
		t.Fatal("forgery accepted")
	}
}
