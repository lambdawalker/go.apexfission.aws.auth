package config

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/lambdawalker/go.apexfission.aws.auth/authorization"
	"testing"
)

func TestRevisionsIsolationAndValidation(t *testing.T) {
	s := Service{Store: NewMemory(), Authorize: authorization.AuthorizerFunc(func(context.Context, authorization.Principal, string, string) error { return nil }), Schemas: map[string]Schema{"sales": {Version: 1, Validate: func(b json.RawMessage) error {
		if string(b) != "{\"limit\":5}" {
			return errors.New("invalid limit")
		}
		return nil
	}}}}
	p := authorization.Principal{Subject: "admin"}
	ctx := context.Background()
	r, e := s.Publish(ctx, p, "dev", "sales", 0, 1, json.RawMessage(`{"limit":5}`))
	if e != nil || r.Revision != 1 {
		t.Fatal(r, e)
	}
	if _, e = s.Publish(ctx, p, "dev", "sales", 0, 1, r.Value); !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	if _, e = s.Store.Get(ctx, "prod", "sales"); !errors.Is(e, ErrNotFound) {
		t.Fatal("environment leak")
	}
	r.Value[0] = 'x'
	got, _ := s.Store.Get(ctx, "dev", "sales")
	if !json.Valid(got.Value) {
		t.Fatal("mutable history")
	}
	if _, e = s.Publish(ctx, p, "dev", "sales", 1, 1, json.RawMessage(`{}`)); e == nil {
		t.Fatal("invalid config accepted")
	}
}
