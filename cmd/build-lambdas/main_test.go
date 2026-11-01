package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRunRejectsInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"-unknown"}, {"-out", ""}, {"extra"}} {
		if err := run(context.Background(), args, io.Discard); err == nil {
			t.Fatalf("accepted invalid arguments %v", args)
		}
	}
}

func TestRunHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := filepath.Join(t.TempDir(), "dist")
	if err := run(ctx, []string{"-out", out}, io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatalf("want cancellation, got %v", err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("canceled build created output: %v", err)
	}
}
