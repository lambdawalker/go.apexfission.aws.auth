package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/lambdawalker/go.apexfission.aws.auth/runtimeconfig"
	"os"
	"path/filepath"
	"testing"
)

func TestSeedRequiresExplicitTargetAndDoesNotInvokeOnInvalidArguments(t *testing.T) {
	calls := 0
	seed := func(context.Context, string, runtimeconfig.Config) error { calls++; return nil }
	var output bytes.Buffer
	if run(context.Background(), nil, &output, seed) != 2 || calls != 0 {
		t.Fatal("implicit target accepted")
	}
	defaults := runtimeconfig.Default("dev")
	path := filepath.Join(t.TempDir(), "defaults.json")
	b, _ := json.Marshal(defaults)
	_ = os.WriteFile(path, b, 0600)
	if run(context.Background(), []string{"-table", "runtime", "-environment", "dev", "-defaults-file", path}, &output, seed) != 0 || calls != 1 {
		t.Fatal("explicit seed notcalled")
	}
	if run(context.Background(), []string{"-table", "runtime", "-environment", "../prod"}, &output, seed) != 2 || calls != 1 {
		t.Fatal("unsafe environment accepted")
	}
}

func TestSeedUsesExportedDisabledCapturePolicyAndRejectsWrongEnvironment(t *testing.T) {
	cfg := runtimeconfig.Default("dev")
	cfg.Features.IDCapture = false
	cfg.Onboarding.IDCapture = runtimeconfig.Disabled
	path := filepath.Join(t.TempDir(), "defaults.json")
	b, _ := json.Marshal(cfg)
	_ = os.WriteFile(path, b, 0600)
	var output bytes.Buffer
	calls := 0
	apply := func(_ context.Context, _ string, c runtimeconfig.Config) error {
		calls++
		if c.Features.IDCapture || c.Onboarding.IDCapture != runtimeconfig.Disabled {
			t.Fatal("deploymentchoiceoverwritten")
		}
		return nil
	}
	if code := run(context.Background(), []string{"-table", "runtime", "-environment", "dev", "-defaults-file", path}, &output, apply); code != 0 {
		t.Fatal(code, output.String())
	}
	if code := run(context.Background(), []string{"-table", "runtime", "-environment", "prod", "-defaults-file", path}, &output, apply); code != 2 || calls != 1 {
		t.Fatal("wrongenvironmentaccepted")
	}
}

type seedFixture struct {
	current runtimeconfig.Config
	calls   int
}

func (f *seedFixture) Seed(_ context.Context, c runtimeconfig.Config) error {
	f.calls++
	if f.calls > 1 {
		return runtimeconfig.ErrConflict
	}
	f.current = c
	return nil
}
func (f *seedFixture) Get(context.Context, string) (runtimeconfig.Config, error) {
	return f.current, nil
}
func TestRerunPreservesExistingOverride(t *testing.T) {
	cfg := runtimeconfig.Default("dev")
	s := &seedFixture{}
	if err := seedConfiguration(context.Background(), s, cfg); err != nil {
		t.Fatal(err)
	}
	s.current.Features.IDCapture = false
	s.current.Onboarding.IDCapture = runtimeconfig.Disabled
	if err := seedConfiguration(context.Background(), s, cfg); err != nil {
		t.Fatal(err)
	}
	if s.current.Features.IDCapture {
		t.Fatal("overridereplaced")
	}
}
