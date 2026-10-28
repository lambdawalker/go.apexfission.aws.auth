// Package auth validates Cognito access tokens independently of authorization.
package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/authorization"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var ErrToken = errors.New("invalid access token")

type KeySource interface {
	Key(context.Context, string) (*rsa.PublicKey, error)
}
type StaticKeys map[string]*rsa.PublicKey

func (s StaticKeys) Key(_ context.Context, k string) (*rsa.PublicKey, error) {
	if v := s[k]; v != nil {
		return v, nil
	}
	return nil, ErrToken
}

type Verifier struct {
	Issuer, ClientID string
	Keys             KeySource
	Now              func() time.Time
}

func (v Verifier) Verify(ctx context.Context, token string) (authorization.Principal, error) {
	bad := authorization.Principal{}
	if v.Issuer == "" || v.ClientID == "" || v.Keys == nil || len(token) > 16384 {
		return bad, ErrToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return bad, ErrToken
	}
	decode := func(s string, out any) error {
		b, e := base64.RawURLEncoding.DecodeString(s)
		if e != nil {
			return e
		}
		return json.Unmarshal(b, out)
	}
	var h struct{ Alg, Kid string }
	if decode(parts[0], &h) != nil || h.Alg != "RS256" || h.Kid == "" {
		return bad, ErrToken
	}
	key, e := v.Keys.Key(ctx, h.Kid)
	if e != nil || key == nil || key.N.BitLen() < 2048 {
		return bad, ErrToken
	}
	sig, e := base64.RawURLEncoding.DecodeString(parts[2])
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if e != nil || rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig) != nil {
		return bad, ErrToken
	}
	var c struct {
		Issuer  string `json:"iss"`
		Client  string `json:"client_id"`
		Subject string `json:"sub"`
		Use     string `json:"token_use"`
		Exp     int64  `json:"exp"`
		Nbf     int64  `json:"nbf"`
		Iat     int64  `json:"iat"`
		Scope   string `json:"scope"`
	}
	if decode(parts[1], &c) != nil {
		return bad, ErrToken
	}
	now := time.Now()
	if v.Now != nil {
		now = v.Now()
	}
	if c.Issuer != v.Issuer || c.Client != v.ClientID || c.Use != "access" || c.Subject == "" || c.Exp <= now.Unix() || c.Nbf > now.Unix() || c.Iat > now.Unix() {
		return bad, ErrToken
	}
	return authorization.Principal{Subject: c.Subject, Issuer: c.Issuer, Scopes: strings.Fields(c.Scope)}, nil
}

// JWKS pins discovery to a configured HTTPS issuer. Unknown kids refresh at most
// once per minute; successful keys expire after one hour. No token-supplied URLs.
type JWKS struct {
	mu                 sync.Mutex
	endpoint           string
	client             *http.Client
	keys               StaticKeys
	expires, lastFetch time.Time
}

func NewJWKS(issuer string) (*JWKS, error) {
	u, e := url.Parse(issuer)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("issuer must be a trusted HTTPS URL")
	}
	return &JWKS{endpoint: strings.TrimRight(issuer, "/") + "/.well-known/jwks.json", client: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (j *JWKS) Key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	now := time.Now()
	if k := j.keys[kid]; k != nil && now.Before(j.expires) {
		return k, nil
	}
	if now.Sub(j.lastFetch) < time.Minute {
		return nil, ErrToken
	}
	j.lastFetch = now
	r, e := http.NewRequestWithContext(ctx, "GET", j.endpoint, nil)
	if e != nil {
		return nil, e
	}
	res, e := j.client.Do(r)
	if e != nil {
		return nil, ErrToken
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, ErrToken
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, 1<<20+1))
	if e != nil || len(b) > 1<<20 {
		return nil, ErrToken
	}
	var doc struct {
		Keys []struct{ Kty, Kid, Use, Alg, N, E string }
	}
	if json.Unmarshal(b, &doc) != nil {
		return nil, ErrToken
	}
	keys := StaticKeys{}
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || k.Use != "sig" || k.Alg != "RS256" || k.Kid == "" {
			continue
		}
		n, ne := base64.RawURLEncoding.DecodeString(k.N)
		eb, ee := base64.RawURLEncoding.DecodeString(k.E)
		if ne != nil || ee != nil || len(eb) > 4 {
			continue
		}
		exp := 0
		for _, v := range eb {
			exp = exp*256 + int(v)
		}
		if exp < 3 || exp%2 == 0 || keys[k.Kid] != nil {
			return nil, ErrToken
		}
		key := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exp}
		if key.N.BitLen() < 2048 {
			return nil, ErrToken
		}
		keys[k.Kid] = key
	}
	if len(keys) == 0 {
		return nil, ErrToken
	}
	j.keys = keys
	j.expires = now.Add(time.Hour)
	return keys.Key(ctx, kid)
}
