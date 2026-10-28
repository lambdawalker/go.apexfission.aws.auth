// Package sales is an application-owned module; it only imports public framework APIs.
package sales

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/admin"
	"github.com/lambdawalker/go.apexfission.aws.auth/authorization"
	"github.com/lambdawalker/go.apexfission.aws.auth/config"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	"github.com/lambdawalker/go.apexfission.aws.auth/router"
	"io"
	"net/http"
)

//go:embed web/*
var assets embed.FS

type Order struct {
	ID     string `json:"id"`
	Amount int64  `json:"amount_cents"`
}
type Store interface {
	List(context.Context, string) ([]Order, error)
	Put(context.Context, string, Order) error
}
type Module struct{ Store Store }

func (Module) Describe() core.Descriptor {
	return core.Descriptor{ID: "sales", Version: "0.1.0", Dependencies: []core.Dependency{{ID: "auth", Major: 0}}}
}
func (Module) Permissions() []string {
	return []string{"sales:read", "sales:write", "sales:admin", "sales:config.read", "sales:config.write"}
}
func (Module) AdminPages() []admin.Page {
	return []admin.Page{{ID: "sales", Title: "Sales", Permission: "sales:admin", Assets: assets, Entry: "web/index.html"}}
}
func (Module) ConfigSchema() config.Schema {
	return config.Schema{Version: 1, Validate: func(b json.RawMessage) error {
		var v struct {
			MaxOrderCents int64 `json:"max_order_cents"`
		}
		if json.Unmarshal(b, &v) != nil || v.MaxOrderCents < 1 || v.MaxOrderCents > 100000000 {
			return fmt.Errorf("max_order_cents must be 1..100000000")
		}
		return nil
	}}
}
func (Module) BuildTargets() []core.BuildTarget {
	return []core.BuildTarget{{Name: "application", Package: "./cmd/api"}, {Name: "challenge", Package: "./cmd/challenge"}}
}
func (m Module) Routes() []router.Route {
	return []router.Route{{Method: "GET", Path: "/sales/orders", Lambda: "application", Permission: "sales:read", Requires: []string{"auth"}, Handler: http.HandlerFunc(m.list)}, {Method: "POST", Path: "/sales/orders", Lambda: "application", Permission: "sales:write", Handler: http.HandlerFunc(m.create)}, {Method: "GET", Path: "/admin/api/extensions/sales/status", Lambda: "admin", Permission: "sales:admin", Admin: true, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"module":"sales","status":"available"}`)
	})}}
}
func (m Module) list(w http.ResponseWriter, r *http.Request) {
	p, ok := authorization.FromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", 401)
		return
	}
	rows, e := m.Store.List(r.Context(), p.Subject)
	if e != nil {
		http.Error(w, "unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rows)
}
func (m Module) create(w http.ResponseWriter, r *http.Request) {
	p, ok := authorization.FromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", 401)
		return
	}
	var in struct {
		Amount int64 `json:"amount_cents"`
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || d.Decode(new(any)) != io.EOF || in.Amount < 1 || in.Amount > 100000000 {
		http.Error(w, "invalid amount_cents", 400)
		return
	}
	id := make([]byte, 16)
	if _, e := rand.Read(id); e != nil {
		http.Error(w, "unavailable", 503)
		return
	}
	order := Order{hex.EncodeToString(id), in.Amount}
	if e := m.Store.Put(r.Context(), p.Subject, order); e != nil {
		http.Error(w, "unavailable", 503)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(order)
}
