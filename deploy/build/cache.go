package build

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	awsenv "github.com/lambdawalker/go.apexfission.aws.auth/deploy/aws"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Options controls bounded builds. Zero Jobs selects four workers. Output is
// written only after workers finish, in target order.
type Options struct {
	Jobs                     int
	ForceRebuild, CleanCache bool
	Output                   io.Writer
}
type receipt struct{ Input, Artifact string }

func buildEnv() []string {
	env := awsenv.WithoutCredentials(os.Environ())
	out := make([]string, 0, len(env)+3)
	for _, v := range env {
		if !strings.HasPrefix(v, "GOOS=") && !strings.HasPrefix(v, "GOARCH=") && !strings.HasPrefix(v, "CGO_ENABLED=") {
			out = append(out, v)
		}
	}
	return append(out, "GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0")
}
func goOutput(ctx context.Context, root string, args ...string) ([]byte, error) {
	cmd := buildCommand(ctx, "go", args...)
	cmd.Dir = root
	cmd.Env = buildEnv()
	b, e := cmd.Output()
	if e != nil {
		return nil, fmt.Errorf("go %s: %w", args[0], e)
	}
	return b, nil
}
func fingerprint(ctx context.Context, root, pkg string) (string, error) {
	env, e := goOutput(ctx, root, "env", "-json")
	if e != nil {
		return "", e
	}
	// All Go environment values are conservative inputs, including toolchain,
	// GOFLAGS, architecture tuning, workspace, experiments and compiler selection.
	h := sha256.New()
	h.Write([]byte("lambda-zip-deflate-bootstrap-0755-v1 build -pgo=off -buildvcs=false -trimpath -tags lambda.norpc -p 1\n"))
	var environment map[string]string
	if e = json.Unmarshal(env, &environment); e != nil {
		return "", e
	}
	// GOGCCFLAGS contains a fresh temporary path on every invocation; Cgo is disabled.
	delete(environment, "GOGCCFLAGS")
	env, _ = json.Marshal(environment)
	h.Write(env)
	files := map[string]bool{}
	overlays := map[string]string{}
	if e = flagFiles(root, environment["GOFLAGS"], files, overlays); e != nil {
		return "", e
	}
	data, e := goOutput(ctx, root, "list", "-buildvcs=false", "-pgo=off", "-deps", "-json", "-tags", "lambda.norpc", pkg)
	if e != nil {
		return "", e
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	for _, key := range []string{"GOWORK", "GOMOD"} {
		if path := environment[key]; filepath.IsAbs(path) && path != "/dev/null" {
			files[path] = true
			if _, e := os.Stat(path + ".sum"); e == nil {
				files[path+".sum"] = true
			}
		}
	}
	// Hash the selected Go driver and its complete tool directory: assembler,
	// compiler, linker, build-id and any other tool the driver may execute.
	driver, e := exec.LookPath("go")
	if e != nil {
		return "", e
	}
	files[driver] = true
	hostExe := ""
	if environment["GOHOSTOS"] == "windows" {
		hostExe = ".exe"
	}
	files[filepath.Join(environment["GOROOT"], "bin", "go"+hostExe)] = true
	for _, dir := range []string{environment["GOTOOLDIR"], filepath.Join(environment["GOROOT"], "pkg", "include")} {
		entries, e := os.ReadDir(dir)
		if os.IsNotExist(e) && dir != environment["GOTOOLDIR"] {
			continue
		}
		if e != nil {
			return "", e
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				files[filepath.Join(dir, entry.Name())] = true
			}
		}
	}

	for dec.More() {
		var p map[string]json.RawMessage
		if e = dec.Decode(&p); e != nil {
			return "", e
		}
		var dir string
		json.Unmarshal(p["Dir"], &dir)
		// Hash metadata and every source/build/embed file in the actual selected closure.
		for _, key := range []string{"ImportPath", "Name", "Module", "DefaultGODEBUG"} {
			h.Write([]byte(key))
			h.Write(p[key])
		}
		for key, raw := range p {
			if strings.HasSuffix(key, "Files") {
				var names []string
				if json.Unmarshal(raw, &names) == nil {
					for _, name := range names {
						if filepath.IsAbs(name) {
							files[name] = true
						} else {
							files[filepath.Join(dir, name)] = true
						}
					}
				}
			}
		}
		var m struct {
			GoMod   string
			Replace *struct{ GoMod string }
		}
		json.Unmarshal(p["Module"], &m)
		if m.GoMod != "" {
			files[m.GoMod] = true
		}
		if m.Replace != nil && m.Replace.GoMod != "" {
			files[m.Replace.GoMod] = true
		}
	}
	for _, name := range []string{"go.mod", "go.sum", "go.work", "go.work.sum"} {
		p := filepath.Join(root, name)
		if _, e := os.Stat(p); e == nil {
			files[p] = true
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		actual := name
		if replacement, ok := overlays[filepath.Clean(name)]; ok {
			if replacement == "" {
				h.Write([]byte(name + "\x00deleted\x00"))
				continue
			}
			actual = replacement
		}
		b, e := os.ReadFile(actual)
		if e != nil {
			return "", e
		}
		h.Write([]byte(name))
		h.Write([]byte{0})
		sum := sha256.Sum256(b)
		h.Write(sum[:])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func digest(path string) (string, error) {
	b, e := os.ReadFile(path)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), e
}
func validCache(archive, record, key string) bool {
	b, e := os.ReadFile(record)
	if e != nil {
		return false
	}
	var r receipt
	if json.Unmarshal(b, &r) != nil || r.Input != key {
		return false
	}
	sum, e := digest(archive)
	if e != nil || sum != r.Artifact {
		return false
	}
	z, e := zip.OpenReader(archive)
	if e != nil {
		return false
	}
	defer z.Close()
	if len(z.File) != 1 || z.File[0].Name != "bootstrap" || z.File[0].Mode().Perm() != 0755 {
		return false
	}
	f, e := z.File[0].Open()
	if e != nil {
		return false
	}
	_, e = io.Copy(io.Discard, f)
	f.Close()
	return e == nil
}
func atomicWrite(path string, b []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".publish-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	if ce := f.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}
func runCached(ctx context.Context, root string, targets []core.BuildTarget, o Options) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	if o.Jobs < 0 {
		return fmt.Errorf("Jobs must be nonnegative")
	}
	if o.Jobs == 0 {
		o.Jobs = 4
	}
	root, e := filepath.Abs(root)
	if e != nil {
		return e
	}
	cache := filepath.Join(root, ".lambda-cache")
	// A root lock also protects cleaning against concurrent readers/builders. Never
	// break a lock based on age or PID: abandoned locks require operator review.
	lock := cache + ".lock"
	if e = os.Mkdir(lock, 0700); e != nil {
		return fmt.Errorf("build cache locked (%s); inspect abandoned locks before removing: %w", lock, e)
	}
	defer os.RemoveAll(lock)
	if o.CleanCache {
		if e = os.RemoveAll(cache); e != nil {
			return e
		}
	}
	if e = os.MkdirAll(cache, 0700); e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Join(root, "dist"), 0755); e != nil {
		return e
	}
	type job struct {
		pkg     string
		indices []int
	}
	jobs := []job{}
	byPackage := map[string]int{}
	for i, t := range targets {
		if j, ok := byPackage[t.Package]; ok {
			jobs[j].indices = append(jobs[j].indices, i)
		} else {
			byPackage[t.Package] = len(jobs)
			jobs = append(jobs, job{t.Package, []int{i}})
		}
	}
	statuses := make([]string, len(targets))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var first error
	var once sync.Once
	var wg sync.WaitGroup
	queue := make(chan job, len(jobs))
	for _, j := range jobs {
		queue <- j
	}
	close(queue)
	for n := 0; n < min(o.Jobs, len(jobs)); n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range queue {
				if ctx.Err() != nil {
					return
				}
				err := func() error {
					key, e := fingerprint(ctx, root, j.pkg)
					if e != nil {
						return e
					}
					archive := filepath.Join(cache, key+".zip")
					record := filepath.Join(cache, key+".json")
					status := "cached"
					if o.ForceRebuild || !validCache(archive, record, key) {
						status = "built"
						tmp, e := os.MkdirTemp(cache, "build-")
						if e != nil {
							return e
						}
						defer os.RemoveAll(tmp)
						binary := filepath.Join(tmp, "bootstrap")
						cmd := buildCommand(ctx, "go", "build", "-pgo=off", "-buildvcs=false", "-p", "1", "-trimpath", "-tags", "lambda.norpc", "-o", binary, j.pkg)
						cmd.Dir = root
						cmd.Env = buildEnv()
						if b, e := cmd.CombinedOutput(); e != nil {
							return fmt.Errorf("build %s: %w: %s", j.pkg, e, b)
						}
						// Refuse to publish under an obsolete key if sources changed while building.
						after, e := fingerprint(ctx, root, j.pkg)
						if e != nil {
							return e
						}
						if after != key {
							return fmt.Errorf("build inputs changed during build: %s", j.pkg)
						}
						if e = PackageLambda(binary, archive); e != nil {
							return e
						}
						sum, e := digest(archive)
						if e != nil {
							return e
						}
						b, _ := json.Marshal(receipt{key, sum})
						if e = atomicWrite(record, b); e != nil {
							return e
						}
					}
					b, e := os.ReadFile(archive)
					if e != nil {
						return e
					}
					for n, i := range j.indices {
						if e = atomicWrite(filepath.Join(root, "dist", targets[i].Name+".zip"), b); e != nil {
							return e
						}
						statuses[i] = status
						if n > 0 && status == "built" {
							statuses[i] = "reused"
						}
					}
					return nil
				}()
				if err != nil {
					once.Do(func() { first = err; cancel() })
					return
				}
			}
		}()
	}
	wg.Wait()
	if o.Output != nil {
		for i, t := range targets {
			if statuses[i] != "" {
				if _, e := fmt.Fprintf(o.Output, "%s: %s\n", t.Name, statuses[i]); e != nil && first == nil {
					first = e
				}
			}
		}
	}
	if first != nil {
		return first
	}
	return ctx.Err()
}

// Go's GOFLAGS fields allow an entire field to be quoted, without shell
// expansions. File-valued flags must hash their contents as well as their name.
func flagFiles(root, flags string, files map[string]bool, overlays map[string]string) error {
	for len(strings.TrimSpace(flags)) > 0 {
		flags = strings.TrimLeft(flags, " \t\r\n")
		var field string
		if flags[0] == '\'' || flags[0] == '"' {
			q := flags[0]
			end := strings.IndexByte(flags[1:], q)
			if end < 0 {
				return fmt.Errorf("unterminated GOFLAGS quote")
			}
			field = flags[1 : end+1]
			flags = flags[end+2:]
		} else {
			end := strings.IndexAny(flags, " \t\r\n")
			if end < 0 {
				field = flags
				flags = ""
			} else {
				field = flags[:end]
				flags = flags[end:]
			}
		}
		name, path, ok := strings.Cut(field, "=")
		if name == "-toolexec" || (name == "-compiler" && path != "gc") {
			return fmt.Errorf("unsupported %s: cached Lambda builds require the standard Go toolchain without wrappers", name)
		}
		if !ok {
			continue
		}
		if name != "-overlay" && name != "-modfile" {
			continue
		}
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		files[path] = true
		if name == "-modfile" {
			sum := strings.TrimSuffix(path, ".mod") + ".sum"
			if _, e := os.Stat(sum); e == nil {
				files[sum] = true
			}
			continue
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		var overlay struct{ Replace map[string]string }
		if e = json.Unmarshal(b, &overlay); e != nil {
			return e
		}
		for logical, replacement := range overlay.Replace {
			if !filepath.IsAbs(logical) {
				logical = filepath.Join(root, logical)
			}
			logical = filepath.Clean(logical)
			overlays[logical] = ""
			if replacement != "" {
				if !filepath.IsAbs(replacement) {
					replacement = filepath.Join(root, replacement)
				}
				files[replacement] = true
				overlays[logical] = replacement
			}
		}
	}
	return nil
}
