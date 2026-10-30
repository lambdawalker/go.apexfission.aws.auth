//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package build

import "os/exec"

// Go's default cancellation kills the process; WaitDelay still bounds pipe wait
// on platforms without supported process-tree APIs.
func configureProcessTree(cmd *exec.Cmd) {}
