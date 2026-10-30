package build

import (
	"bytes"
	"context"
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheDependencyAndArtifactIntegrity(t *testing.T) {
	root := t.TempDir()
	write := func(p, s string) {
		t.Helper()
		if e := os.WriteFile(filepath.Join(root, p), []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	write("go.mod", "module fixture\n\ngo 1.26.6\n")
	os.Mkdir(filepath.Join(root, "cmd"), 0700)
	os.Mkdir(filepath.Join(root, "dep"), 0700)
	write("cmd/main.go", "package main\nimport \"fixture/dep\"\nfunc main(){println(dep.Value)}")
	write("dep/value.go", "package dep\nconst Value=1")
	targets := []core.BuildTarget{{Name: "a", Package: "./cmd"}, {Name: "b", Package: "./cmd"}}
	var out bytes.Buffer
	run := func() {
		t.Helper()
		out.Reset()
		if e := LambdasWithOptions(context.Background(), root, targets, Options{Jobs: 2, Output: &out}); e != nil {
			t.Fatal(e)
		}
	}
	run()
	if strings.Count(out.String(), "built") != 1 || !strings.Contains(out.String(), "reused") {
		t.Fatal(out.String())
	}
	run()
	if strings.Count(out.String(), "cached") != 2 {
		t.Fatal(out.String())
	}
	write("dep/value.go", "package dep\nconst Value=2")
	run()
	if !strings.Contains(out.String(), "built") {
		t.Fatal(out.String())
	}
	entries, _ := filepath.Glob(filepath.Join(root, ".lambda-cache", "*.zip"))
	for _, p := range entries {
		os.WriteFile(p, []byte("tampered"), 0600)
	}
	run()
	if !strings.Contains(out.String(), "built") {
		t.Fatal(out.String())
	}

	before, e := os.ReadFile(filepath.Join(root, "dist", "a.zip"))
	if e != nil {
		t.Fatal(e)
	}
	for _, options := range []Options{{Jobs: 1, ForceRebuild: true, Output: &out}, {Jobs: 1, CleanCache: true, Output: &out}} {
		out.Reset()
		if e := LambdasWithOptions(context.Background(), root, targets, options); e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(out.String(), "built") {
			t.Fatal("option did not rebuild", out.String())
		}
		after, e := os.ReadFile(filepath.Join(root, "dist", "a.zip"))
		if e != nil || !bytes.Equal(before, after) {
			t.Fatal("nondeterministic archive", e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := LambdasWithOptions(ctx, root, targets, Options{Jobs: 2}); e == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestFingerprintSelectedClosure(t *testing.T) {
	root := t.TempDir()
	write := func(p, s string) {
		t.Helper()
		if e := os.WriteFile(filepath.Join(root, p), []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	write("go.mod", "module fixture\n\ngo 1.26.6\nrequire example.org/dep v0.0.0\nreplace example.org/dep => ./dep\n")
	os.Mkdir(filepath.Join(root, "cmd"), 0700)
	os.Mkdir(filepath.Join(root, "dep"), 0700)
	write("cmd/main.go", "package main\nimport (_ \"embed\"; \"example.org/dep\")\n//go:embed asset.txt\nvar asset string\nfunc main(){println(asset,dep.Value)}")
	write("cmd/asset.txt", "one")
	write("dep/go.mod", "module example.org/dep\ngo 1.26.6\n")
	write("dep/value.go", "package dep\nconst Value=1")
	key := func() string {
		t.Helper()
		s, e := fingerprint(context.Background(), root, "./cmd")
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	a := key()
	if a != key() {
		t.Fatal("unstable fingerprint")
	}
	write("unrelated.txt", "ignored")
	if a != key() {
		t.Fatal("unrelated file invalidates")
	}
	write("cmd/asset.txt", "two")
	b := key()
	if a == b {
		t.Fatal("embed ignored")
	}
	write("dep/value.go", "package dep\nconst Value=2")
	c := key()
	if b == c {
		t.Fatal("replacement dependency ignored")
	}
	t.Setenv("GOFLAGS", "-ldflags=-s")
	if c == key() {
		t.Fatal("GOFLAGS ignored")
	}
}
func TestCacheLockFailsClosed(t *testing.T) {
	root := t.TempDir()
	lock := filepath.Join(root, ".lambda-cache.lock")
	os.Mkdir(lock, 0700)
	if e := LambdasWithOptions(context.Background(), root, nil, Options{CleanCache: true}); e == nil || !strings.Contains(e.Error(), "locked") {
		t.Fatal(e)
	}
	if _, e := os.Stat(lock); e != nil {
		t.Fatal("existing lock removed")
	}
}
func TestParallelBuildCancellation(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\ngo 1.26.6\n"), 0600)
	for _, p := range []string{"a", "b", "c"} {
		os.Mkdir(filepath.Join(root, p), 0700)
		os.WriteFile(filepath.Join(root, p, "main.go"), []byte("package main\nfunc main(){ undefined() }"), 0600)
	}
	targets := []core.BuildTarget{{Name: "a", Package: "./a"}, {Name: "b", Package: "./b"}, {Name: "c", Package: "./c"}}
	if e := LambdasWithOptions(context.Background(), root, targets, Options{Jobs: 2}); e == nil {
		t.Fatal("invalid builds accepted")
	}
	if _, e := os.Stat(filepath.Join(root, ".lambda-cache.lock")); !os.IsNotExist(e) {
		t.Fatal("lock leaked", e)
	}
	matches, _ := filepath.Glob(filepath.Join(root, ".lambda-cache", "*.json"))
	if len(matches) > 0 {
		t.Fatal("failed build published receipt")
	}
}

func TestFingerprintOverlayContent(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\ngo 1.26.6\n"), 0600)
	os.Mkdir(filepath.Join(root, "cmd"), 0700)
	source := filepath.Join(root, "cmd", "main.go")
	replacement := filepath.Join(root, "replacement.go")
	os.WriteFile(source, []byte("package main\nfunc main(){}"), 0600)
	os.WriteFile(replacement, []byte("package main\nfunc main(){println(1)}"), 0600)
	overlay := filepath.Join(root, "overlay.json")
	data := fmt.Sprintf(`{"Replace":{%q:%q}}`, source, replacement)
	os.WriteFile(overlay, []byte(data), 0600)
	t.Setenv("GOFLAGS", "-overlay="+overlay)
	a, e := fingerprint(context.Background(), root, "./cmd")
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(replacement, []byte("package main\nfunc main(){println(2)}"), 0600)
	b, e := fingerprint(context.Background(), root, "./cmd")
	if e != nil {
		t.Fatal(e)
	}
	if a == b {
		t.Fatal("overlay content ignored")
	}
}

func TestFingerprintRejectsMutableToolWrapper(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\ngo 1.26.6\n"), 0600)
	os.Mkdir(filepath.Join(root, "cmd"), 0700)
	os.WriteFile(filepath.Join(root, "cmd", "main.go"), []byte("package main\nfunc main(){}"), 0600)
	wrapper := filepath.Join(root, "wrapper")
	t.Setenv("GOFLAGS", "-toolexec="+wrapper)
	for _, content := range []string{"first version", "changed at same path"} {
		os.WriteFile(wrapper, []byte(content), 0700)
		if _, e := fingerprint(context.Background(), root, "./cmd"); e == nil || !strings.Contains(e.Error(), "unsupported -toolexec") {
			t.Fatalf("mutable wrapper accepted: %v", e)
		}
	}
}
func TestOverlayIntroducesAndDeletesSource(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\ngo 1.26.6\n"), 0600)
	os.Mkdir(filepath.Join(root, "cmd"), 0700)
	source := filepath.Join(root, "cmd", "main.go")
	os.WriteFile(source, []byte("package main\nfunc main(){println(value)}"), 0600)
	removed := filepath.Join(root, "cmd", "removed.go")
	os.WriteFile(removed, []byte("package main\nconst value=99"), 0600)
	added := filepath.Join(root, "cmd", "generated.go")
	backing := filepath.Join(root, "backing.go")
	os.WriteFile(backing, []byte("package main\nconst value=1"), 0600)
	overlay := filepath.Join(root, "overlay.json")
	os.WriteFile(overlay, []byte(fmt.Sprintf(`{"Replace":{%q:%q,%q:""}}`, added, backing, removed)), 0600)
	t.Setenv("GOFLAGS", "-overlay="+overlay)
	targets := []core.BuildTarget{{Name: "overlay", Package: "./cmd"}}
	if e := Lambdas(context.Background(), root, targets); e != nil {
		t.Fatal(e)
	}
	a, e := fingerprint(context.Background(), root, "./cmd")
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(backing, []byte("package main\nconst value=2"), 0600)
	b, e := fingerprint(context.Background(), root, "./cmd")
	if e != nil || a == b {
		t.Fatal("virtual source content not tracked", e)
	}
}

func TestPGOExplicitlyDisabled(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\ngo 1.26.6\n"), 0600)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main; func main(){}"), 0600)
	profile := filepath.Join(root, "default.pgo")
	os.WriteFile(profile, []byte("invalid automatic profile"), 0600)
	for _, flags := range []string{"", "-pgo=" + profile} {
		t.Setenv("GOFLAGS", flags)
		if err := LambdasWithOptions(context.Background(), root, []core.BuildTarget{{Name: "main", Package: "./"}}, Options{ForceRebuild: true}); err != nil {
			t.Fatalf("PGO must be disabled for metadata and actual build: %v", err)
		}
	}
}
