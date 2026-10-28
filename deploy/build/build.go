package build

import (
	"context"
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	awsenv "github.com/lambdawalker/go.apexfission.aws.auth/deploy/aws"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

func Lambdas(ctx context.Context, root string, targets []core.BuildTarget) error {
	seen := map[string]bool{}
	for _, t := range targets {
		if !core.IDPattern.MatchString(t.Name) || seen[t.Name] || !strings.HasPrefix(t.Package, "./") || strings.Contains(t.Package, "..") || strings.ContainsAny(t.Package, "\\:") {
			return fmt.Errorf("invalid build target %s", t.Name)
		}
		seen[t.Name] = true
		real, e := filepath.EvalSymlinks(filepath.Join(root, t.Package))
		if e != nil {
			return e
		}
		abs, e := filepath.Abs(root)
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(abs, real)
		if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("build escapes application root")
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	queue := make(chan core.BuildTarget, len(targets))
	for _, t := range targets {
		queue <- t
	}
	close(queue)
	var wg sync.WaitGroup
	var once sync.Once
	var first error
	for i := 0; i < min(4, len(targets)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range queue {
				if ctx.Err() != nil {
					return
				}
				e := buildOne(ctx, root, t)
				if e != nil {
					once.Do(func() { first = e; cancel() })
					return
				}
			}
		}()
	}
	wg.Wait()
	if first != nil {
		return first
	}
	return ctx.Err()
}
func buildOne(ctx context.Context, root string, t core.BuildTarget) error {
	dir, e := os.MkdirTemp("", "apexfission-build-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	binary := filepath.Join(dir, "bootstrap")
	cmd := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-p", "2", "-trimpath", "-tags", "lambda.norpc", "-o", binary, t.Package)
	cmd.Dir = root
	env := awsenv.WithoutCredentials(os.Environ())
	filtered := env[:0]
	for _, v := range env {
		if !strings.HasPrefix(v, "GOOS=") && !strings.HasPrefix(v, "GOARCH=") && !strings.HasPrefix(v, "CGO_ENABLED=") {
			filtered = append(filtered, v)
		}
	}
	cmd.Env = append(filtered, "GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0")
	if out, e := cmd.CombinedOutput(); e != nil {
		return fmt.Errorf("build %s: %w: %s", t.Name, e, out)
	}
	dist := filepath.Join(root, "dist")
	if e = os.MkdirAll(dist, 0755); e != nil {
		return e
	}
	return PackageLambda(binary, filepath.Join(dist, t.Name+".zip"))
}
