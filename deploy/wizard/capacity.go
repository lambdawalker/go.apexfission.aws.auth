package wizard

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Only increases count: reductions cannot be credited because Pulumi can update
// the three capture functions concurrently. Preserve 100 unreserved executions.
func captureCapacity(requested, total, unreserved int, existing [3]int) (int, int, error) {
	if requested < 1 || requested > int(^uint(0)>>1)/3 || total < 0 || unreserved < 0 || unreserved > total {
		return 0, 0, errors.New("invalid Lambda concurrency response")
	}
	additional := 0
	for _, v := range existing {
		if v < 0 || v > total {
			return 0, 0, errors.New("invalid current Lambda reservation")
		}
		additional += max(0, requested-v)
	}
	return additional, max(0, unreserved-min(100, total)), nil
}
func (x *execution) capacity(config map[string]configValue) error {
	return x.capacityCheck(config, true)
}
func (x *execution) capacityCheck(config map[string]configValue, deployed bool) error {
	production := (x.c.Environment == "prod" || x.c.Environment == "production") || x.c.EnvironmentClass == "production" || config["attestra-backend:environmentClass"].Value == "production"
	if production && (x.c.CaptureReserved < 1 || x.c.CaptureWorker < 2) {
		return errors.New("production requires explicit positive --capture-reserved-concurrency and --capture-worker-max-concurrency; no cloud update performed")
	}
	if x.c.CaptureReserved < 1 {
		x.env = withEnv(x.env, "ATTESTRA_CAPTURE_RESERVED_CONCURRENCY", "-1")
		return nil
	}
	var state struct {
		Deployment *struct {
			Resources []struct {
				URN, Type, ID string
				Delete        bool
			}
		}
	}
	var e error
	if deployed {
		raw, readErr := x.run("pulumi", "stack", "export", "--stack", x.stack)
		if readErr != nil {
			return readErr
		}
		if json.Unmarshal(raw, &state) != nil || state.Deployment == nil {
			return errors.New("cannot inspect existing capture Lambda identities")
		}
	}
	existing := [3]int{}
	if state.Deployment != nil {
		for _, r := range state.Deployment.Resources {
			if r.Type != "aws:lambda/function:Function" || r.ID == "" || r.Delete {
				continue
			}
			for i, name := range []string{"capture", "capture-worker", "capture-dispatcher"} {
				if !strings.HasSuffix(r.URN, "::"+name) {
					continue
				}
				var current struct{ ReservedConcurrentExecutions *int }
				if e = x.aws(&current, "lambda", "get-function-concurrency", "--function-name", r.ID); e != nil {
					return e
				}
				if current.ReservedConcurrentExecutions != nil {
					existing[i] = *current.ReservedConcurrentExecutions
				}
			}
		}
	}
	var account struct {
		AccountLimit struct{ ConcurrentExecutions, UnreservedConcurrentExecutions *int }
	}
	if e = x.aws(&account, "lambda", "get-account-settings"); e != nil {
		return e
	}
	if account.AccountLimit.ConcurrentExecutions == nil || account.AccountLimit.UnreservedConcurrentExecutions == nil {
		return errors.New("invalid Lambda account quota response")
	}
	required, available, e := captureCapacity(x.c.CaptureReserved, *account.AccountLimit.ConcurrentExecutions, *account.AccountLimit.UnreservedConcurrentExecutions, existing)
	if e != nil {
		return e
	}
	effective := x.c.CaptureReserved
	if required > available {
		if production {
			return fmt.Errorf("production capture needs %d additional reservations, %d available; request quota before deployment", required, available)
		}
		effective = -1
		fmt.Fprintln(x.o.Output, "Capture reservation exceeds available quota; using shared account concurrency for this test deployment.")
	}
	x.env = withEnv(x.env, "ATTESTRA_CAPTURE_RESERVED_CONCURRENCY", fmt.Sprint(effective))
	return nil
}

// Inspect quota and existing target pins before a both-scope shared update.
func (x *execution) preflightBackend() error {
	if e := x.locateStack(false); e != nil {
		return e
	}
	b, e := x.run("pulumi", "stack", "ls", "--json")
	if e != nil {
		return e
	}
	var stacks []struct{ Name string }
	if json.Unmarshal(b, &stacks) != nil {
		return errors.New("cannot safely inspect backend stacks for preflight")
	}
	exists := false
	for _, s := range stacks {
		exists = exists || s.Name == x.stack || strings.HasSuffix(s.Name, "/"+x.stack)
	}
	config := map[string]configValue{}
	if exists {
		if e = x.selectStack(false, false); e != nil {
			return e
		}
		config, e = x.config()
		if e != nil {
			return e
		}
		if e = x.checkPins(config, true); e != nil {
			return e
		}
	}
	return x.capacityCheck(config, exists)
}
