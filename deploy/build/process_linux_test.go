package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCancellationKillsActiveDescendant(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "child.pid")
	fake := filepath.Join(root, "go")
	script := fmt.Sprintf("#!/bin/sh\nsleep 30 &\necho $! > %q\nwait\n", marker)
	if e := os.WriteFile(fake, []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", root+":"+os.Getenv("PATH"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, e := goOutput(ctx, root, "env"); done <- e }()
	var pid int
	deadline := time.After(5 * time.Second)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
ready:
	for {
		select {
		case <-deadline:
			t.Fatal("child never started")
		case <-tick.C:
			b, e := os.ReadFile(marker)
			if e == nil {
				pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
				if pid > 0 {
					break ready
				}
			}
		}
	}
	defer syscall.Kill(pid, syscall.SIGKILL)
	cancel()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("cancellation succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation blocked on descendant output pipe")
	}
	// Linux may briefly retain a killed orphan as a zombie; it must not be running.
	b, e := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if e == nil {
		fields := strings.Fields(string(b))
		if len(fields) > 2 && fields[2] != "Z" {
			t.Fatalf("descendant survived cancellation: %s", b)
		}
	}
}

func TestFingerprintIncludesAssemblerIdentity(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "bin"), 0700)
	tools := filepath.Join(root, "tools")
	os.Mkdir(tools, 0700)
	for _, name := range []string{"compile", "link", "asm"} {
		os.WriteFile(filepath.Join(tools, name), []byte("original"), 0700)
	}
	os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main(){}"), 0600)
	env := fmt.Sprintf(`{"GOROOT":%q,"GOTOOLDIR":%q,"GOHOSTOS":"linux"}`, root, tools)
	list := fmt.Sprintf(`{"Dir":%q,"ImportPath":"fixture","GoFiles":["main.go"]}`, root)
	fake := filepath.Join(root, "bin", "go")
	script := fmt.Sprintf("#!/bin/sh\ncase $1 in\nenv) echo '%s';;\nlist) echo '%s';;\nesac\n", env, list)
	os.WriteFile(fake, []byte(script), 0700)
	t.Setenv("PATH", filepath.Join(root, "bin")+":"+os.Getenv("PATH"))
	a, e := fingerprint(context.Background(), root, "./cmd")
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(tools, "asm"), []byte("modified assembler"), 0700)
	b, e := fingerprint(context.Background(), root, "./cmd")
	if e != nil || a == b {
		t.Fatal("assembler replacement did not invalidate fingerprint", e)
	}
}
