package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/deploy/build"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Runner interface {
	Run(context.Context, string, []string, string, ...string) ([]byte, error)
}
type Process struct{}

func (Process) Run(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, name, args...)
	c.Dir = dir
	c.Env = env
	out, e := c.CombinedOutput()
	if e != nil {
		return nil, fmt.Errorf("%s failed: %w", name, e)
	}
	return out, nil
}

type Options struct {
	Confirm                       string
	AllowDestroy, AllowProduction bool
}
type Step struct {
	ID  string
	Run func(context.Context) error
}
type Engine struct {
	Manifest      Manifest
	Root          string
	Runner        Runner
	Before, After []Step
}

func (e Engine) Execute(ctx context.Context, op, environment string, o Options) error {
	if err := e.Manifest.Validate(); err != nil {
		return err
	}
	if op == "build" {
		return build.Lambdas(ctx, e.Root, e.Manifest.Builds)
	}
	if op != "preview" && op != "deploy" && op != "status" && op != "teardown" {
		return fmt.Errorf("unknown operation %s", op)
	}
	target, ok := e.Manifest.Environments[environment]
	if !ok {
		return fmt.Errorf("unknown environment %s", environment)
	}
	confirm := e.Manifest.Application + "/" + environment + "/" + target.Account + "/" + target.Region + "/" + target.Stack + "/" + target.Backend
	if op == "teardown" && (!o.AllowDestroy || o.Confirm != confirm || (target.Production || environment == "prod" || environment == "production") && !o.AllowProduction) {
		return fmt.Errorf("teardown requires --allow-destroy --confirm %s and --allow-production for production", confirm)
	}
	if e.Runner == nil {
		e.Runner = Process{}
	}
	dir := filepath.Join(e.Root, target.Directory)
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	root, err := filepath.Abs(e.Root)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("stack directory escapes application root")
	}
	env := os.Environ()
	for _, k := range []string{"AWS_REGION", "AWS_DEFAULT_REGION", "PULUMI_BACKEND_URL"} {
		clean := env[:0]
		for _, v := range env {
			if !strings.HasPrefix(v, k+"=") {
				clean = append(clean, v)
			}
		}
		env = clean
	}
	env = append(env, "AWS_REGION="+target.Region, "AWS_DEFAULT_REGION="+target.Region, "PULUMI_BACKEND_URL="+target.Backend)
	run := func(name string, args ...string) ([]byte, error) { return e.Runner.Run(ctx, dir, env, name, args...) }
	identity, err := run("aws", "sts", "get-caller-identity", "--output", "json")
	if err != nil {
		return err
	}
	var caller struct{ Account string }
	if json.Unmarshal(identity, &caller) != nil || caller.Account != target.Account {
		return fmt.Errorf("AWS account mismatch; expected %s", target.Account)
	}
	if _, err = run("pulumi", "stack", "select", target.Stack, "--non-interactive"); err != nil {
		return fmt.Errorf("select existing stack (initialize explicitly first): %w", err)
	}
	rawConfig, err := run("pulumi", "config", "--json", "--stack", target.Stack)
	if err != nil {
		return err
	}
	var config map[string]struct {
		Value string `json:"value"`
	}
	if json.Unmarshal(rawConfig, &config) != nil {
		return fmt.Errorf("invalid Pulumi configuration")
	}
	for key := range config {
		if strings.HasPrefix(key, "aws:") && key != "aws:region" && key != "aws:allowedAccountIds" {
			return fmt.Errorf("AWS provider override %s requires review", key)
		}
	}
	var allowed []string
	if config["aws:region"].Value != target.Region || json.Unmarshal([]byte(config["aws:allowedAccountIds"].Value), &allowed) != nil || len(allowed) != 1 || allowed[0] != target.Account {
		return fmt.Errorf("stack must pin aws:region and aws:allowedAccountIds to the manifest target before execution")
	}

	if op == "status" {
		out, err := run("pulumi", "stack", "output", "--json", "--stack", target.Stack)
		if err == nil {
			fmt.Println(string(out))
		}
		return err
	}
	receiptDir := filepath.Join(e.Root, ".apexfission")
	if err = os.MkdirAll(receiptDir, 0700); err != nil {
		return err
	}
	lock := filepath.Join(receiptDir, environment+".lock")
	f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("operation locked; inspect previous operation before recovering: %w", err)
	}
	f.Close()
	defer os.Remove(lock)
	record := func(stage string) error {
		b, _ := json.Marshal(map[string]any{"application": e.Manifest.Application, "environment": environment, "account": target.Account, "operation": op, "stage": stage, "at": time.Now().UTC()})
		return os.WriteFile(filepath.Join(receiptDir, environment+".json"), b, 0600)
	}
	if err = record("started"); err != nil {
		return err
	}
	if op != "teardown" {
		if err = build.Lambdas(ctx, e.Root, e.Manifest.Builds); err != nil {
			return err
		}
	}
	if op == "deploy" {
		for _, step := range e.Before {
			if err = step.Run(ctx); err != nil {
				return fmt.Errorf("before step %s: %w", step.ID, err)
			}
		}
	}
	args := []string{"preview", "--stack", target.Stack, "--non-interactive", "--diff"}
	if op == "teardown" {
		args = append(args, "--destroy")
	}
	out, err := run("pulumi", args...)
	if err != nil {
		return err
	}
	fmt.Print(string(out))
	if err = record("previewed"); err != nil {
		return err
	}
	if op == "preview" {
		return record("complete")
	}
	action := "up"
	if op == "teardown" {
		action = "destroy"
	}
	out, err = run("pulumi", action, "--stack", target.Stack, "--non-interactive", "--yes")
	if err != nil {
		return err
	}
	fmt.Print(string(out))
	if op == "deploy" {
		for _, step := range e.After {
			if err = step.Run(ctx); err != nil {
				return fmt.Errorf("after step %s: %w", step.ID, err)
			}
		}
	}
	return record("complete")
}
