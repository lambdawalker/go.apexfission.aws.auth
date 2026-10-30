package build

import (
	"context"
	"os/exec"
	"strconv"
	"time"
)

func configureProcessTree(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		// /T terminates descendants; /F also handles non-console compiler processes.
		if e := exec.CommandContext(ctx, "taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").Run(); e != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}
