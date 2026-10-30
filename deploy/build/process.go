package build

import (
	"context"
	"os/exec"
	"time"
)

// Every Go invocation owns its process tree. WaitDelay also bounds pipe cleanup
// if an unrelated process has inherited a tool's output descriptor.
func buildCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 2 * time.Second
	configureProcessTree(cmd)
	return cmd
}
