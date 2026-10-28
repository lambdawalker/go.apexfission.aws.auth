// Package config stores namespaced, revisioned application configuration.
package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/authorization"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	"sync"
	"time"
)

var ErrConflict = errors.New("revision conflict")
var ErrNotFound = errors.New("configuration not found")

type Revision struct {
	Environment   string          `json:"environment"`
	Namespace     string          `json:"namespace"`
	Revision      int64           `json:"revision"`
	SchemaVersion int             `json:"schema_version"`
	Actor         string          `json:"actor"`
	At            time.Time       `json:"at"`
	Value         json.RawMessage `json:"value"`
}
type Schema struct {
	Version  int
	Validate func(json.RawMessage) error
	Migrate  func(int, json.RawMessage) (json.RawMessage, error)
}
type Module interface {
	core.Module
	ConfigSchema() Schema
}
type Store interface {
	Get(context.Context, string, string) (Revision, error)
	Publish(context.Context, int64, Revision) error
	History(context.Context, string, string) ([]Revision, error)
}
type Service struct {
	Store     Store
	Schemas   map[string]Schema
	Authorize authorization.Authorizer
}

func (s Service) Publish(ctx context.Context, p authorization.Principal, env, ns string, expected int64, version int, value json.RawMessage) (Revision, error) {
	var zero Revision
	if !core.IDPattern.MatchString(env) || !core.IDPattern.MatchString(ns) || expected < 0 || p.Subject == "" {
		return zero, fmt.Errorf("invalid publication target")
	}
	if s.Authorize == nil || s.Authorize.Authorize(ctx, p, env, ns+":config.write") != nil {
		return zero, authorization.ErrDenied
	}
	schema, ok := s.Schemas[ns]
	if !ok || schema.Validate == nil || schema.Version < 1 {
		return zero, fmt.Errorf("unknown schema %s", ns)
	}
	if version != schema.Version {
		if schema.Migrate == nil || version < 1 || version > schema.Version {
			return zero, fmt.Errorf("unsupported schema version")
		}
		var e error
		value, e = schema.Migrate(version, value)
		if e != nil {
			return zero, e
		}
	}
	if len(value) > 128*1024 || !json.Valid(value) {
		return zero, fmt.Errorf("invalid configuration JSON")
	}
	if e := schema.Validate(value); e != nil {
		return zero, e
	}
	r := Revision{env, ns, expected + 1, schema.Version, p.Subject, time.Now().UTC(), append(json.RawMessage(nil), value...)}
	var publishErr error
	if guard, ok := s.Authorize.(interface{ GrantTable() string }); ok {
		store, ok := s.Store.(interface {
			PublishGuarded(context.Context, int64, Revision, string, string) error
		})
		if !ok {
			return zero, fmt.Errorf("durable grants require atomic guarded publication")
		}
		publishErr = store.PublishGuarded(ctx, expected, r, guard.GrantTable(), ns+":config.write")
	} else {
		publishErr = s.Store.Publish(ctx, expected, r)
	}
	if e := publishErr; e != nil {
		return zero, e
	}
	return r, nil
}

// Memory is a concurrency-safe local/test store. Use Dynamo for durable publication.
type Memory struct {
	mu   sync.RWMutex
	rows map[string][]Revision
}

func NewMemory() *Memory        { return &Memory{rows: map[string][]Revision{}} }
func key(env, ns string) string { return env + "\x00" + ns }
func clone(r Revision) Revision { r.Value = append(json.RawMessage(nil), r.Value...); return r }
func (m *Memory) Get(_ context.Context, e, n string) (Revision, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rows := m.rows[key(e, n)]
	if len(rows) == 0 {
		return Revision{}, ErrNotFound
	}
	return clone(rows[len(rows)-1]), nil
}
func (m *Memory) Publish(_ context.Context, expected int64, r Revision) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key(r.Environment, r.Namespace)
	if int64(len(m.rows[k])) != expected || r.Revision != expected+1 {
		return ErrConflict
	}
	m.rows[k] = append(m.rows[k], clone(r))
	return nil
}
func (m *Memory) History(_ context.Context, e, n string) ([]Revision, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []Revision{}
	for _, r := range m.rows[key(e, n)] {
		out = append(out, clone(r))
	}
	return out, nil
}
