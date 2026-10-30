package build

import (
	"context"
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	"path/filepath"
	"strings"
)

func Lambdas(ctx context.Context, root string, targets []core.BuildTarget) error {
	return LambdasWithOptions(ctx, root, targets, Options{})
}

func LambdasWithOptions(ctx context.Context, root string, targets []core.BuildTarget, options Options) error {
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
	return runCached(ctx, root, targets, options)
}
