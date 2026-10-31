package wizard

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	"github.com/muesli/cancelreader"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScopedChoicesAndExplicitEmpty(t *testing.T) {
	root := t.TempDir()
	c := defaults()
	c.Account = "123456789012"
	c.StateBackend = "s3://state"
	c.Profile = "dev-profile"
	s := saved{Version: 1}
	s.remember(c)
	prod := c
	prod.Environment = "prod"
	prod.Profile = "prod-profile"
	s.remember(prod)
	if err := save(filepath.Join(root, ".attestra", "wizard.json"), s); err != nil {
		t.Fatal(err)
	}
	_, got, _, err := parse(Options{Root: root, Output: io.Discard, Args: []string{"--environment", "dev", "--scope", "all", "--profile", ""}})
	if err != nil || got.Profile != "" || got.Environment != "dev" || got.Scope != "both" {
		t.Fatalf("wrong scoped choice: %+v %v", got, err)
	}
	_, got, _, err = parse(Options{Root: root, Output: io.Discard, Args: []string{"--environment", "dev", "--scope", "all"}})
	if err != nil || got.Profile != "dev-profile" {
		t.Fatalf("wrong scope restore %+v %v", got, err)
	}
}
func TestBuildOperationAvoidsCloudAndPassesSelectedTargets(t *testing.T) {
	o, r := fixture(t)
	o.Args = append(o.Args, "--operation", "build", "--scope", "admin-index", "--build-jobs", "2", "--force-rebuild")
	called := false
	o.Build = func(context.Context, string, []core.BuildTarget) error { called = true; return nil }
	if err := Run(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if !called || len(r.calls) != 0 {
		t.Fatal("build must not contact cloud")
	}
}
func TestDestructivePreviewCannotUseUnattendedYes(t *testing.T) {
	x := execution{ctx: context.Background(), approved: true, o: Options{Input: strings.NewReader(""), Output: io.Discard}}
	if x.confirmPreview("1 delete", true) == nil {
		t.Fatal("destructive autoapproval")
	}
	if x.confirmPreview("1 create", false) != nil {
		t.Fatal("non-destructive explicit approval rejected")
	}
}
func TestSharedInteractiveSkipsBackendQuestions(t *testing.T) {
	o := Options{Root: t.TempDir(), Output: io.Discard, reader: bufio.NewReader(strings.NewReader("admin-index\nnext\npreview\n123456789012\nus-east-1\ns3://state\nattestra.com\n\n\n"))}
	_, c, _, err := parse(o)
	if err != nil {
		t.Fatal(err)
	}
	if c.SenderDomain != "" || c.Scope != "shared" {
		t.Fatalf("%+v", c)
	}
}

func TestFirstCheckoutBuildNeedsNoCloudConfiguration(t *testing.T) {
	called := false
	err := Run(context.Background(), Options{Root: t.TempDir(), Output: io.Discard, Args: []string{"--operation", "build"}, Build: func(context.Context, string, []core.BuildTarget) error { called = true; return nil }})
	if err != nil || !called {
		t.Fatalf("build failed: %v", err)
	}
}
func TestBuildControlsAreInvocationOnly(t *testing.T) {
	s := saved{Version: 1}
	c := defaults()
	c.ForceRebuild = true
	c.CleanCache = true
	s.remember(c)
	if s.Choices.ForceRebuild || s.Choices.CleanCache {
		t.Fatal("transient controls persisted")
	}
}

type approvalRead struct {
	io.Reader
	cancel func()
	closed bool
}

func (r *approvalRead) Cancel() bool {
	if r.cancel != nil {
		r.cancel()
	}
	return true
}
func (r *approvalRead) Close() error { r.closed = true; return nil }
func TestCountdownReadsAndFakeClock(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		ok          bool
	}{{"enter", "\n", true}, {"yes", "yes\n", true}, {"no", "no\n", false}, {"escape", "\x1b", false}, {"interrupt", "\x03", false}, {"eof", "", false}} {
		t.Run(tc.name, func(t *testing.T) {
			r := &approvalRead{Reader: strings.NewReader(tc.input)}
			x := execution{ctx: context.Background(), o: Options{Output: io.Discard, ApprovalReader: func() (cancelreader.CancelReader, error) { return r, nil }, ApprovalTicker: func() (<-chan time.Time, func()) { return make(chan time.Time), func() {} }}}
			err := x.confirmPreview("1 create", false)
			if (err == nil) != tc.ok || !r.closed {
				t.Fatalf("err=%v closed=%v", err, r.closed)
			}
		})
	}
	t.Run("timeout", func(t *testing.T) {
		pr, pw := io.Pipe()
		defer pw.Close()
		r := &approvalRead{Reader: pr, cancel: func() { pr.Close() }}
		ticks := make(chan time.Time, 5)
		for i := 0; i < 5; i++ {
			ticks <- time.Time{}
		}
		var output bytes.Buffer
		stopped := false
		x := execution{ctx: context.Background(), o: Options{Output: &output, ApprovalReader: func() (cancelreader.CancelReader, error) { return r, nil }, ApprovalTicker: func() (<-chan time.Time, func()) { return ticks, func() { stopped = true } }}}
		if err := x.confirmPreview("1 create", false); err != nil {
			t.Fatal(err)
		}
		if !stopped || !r.closed {
			t.Fatal("cleanup missing")
		}
		for _, number := range []string{"5...", "4...", "3...", "2...", "1..."} {
			if !strings.Contains(output.String(), number) {
				t.Fatal("missing countdown", output.String())
			}
		}
	})
}
func TestApprovalContextCancelsBlockedReader(t *testing.T) {
	for _, preview := range []string{"1 create", "1 delete"} {
		t.Run(preview, func(t *testing.T) {
			pr, pw := io.Pipe()
			defer pw.Close()
			r := &approvalRead{Reader: pr, cancel: func() { pr.Close() }}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			x := execution{ctx: ctx, o: Options{Output: io.Discard, ApprovalReader: func() (cancelreader.CancelReader, error) { return r, nil }, ApprovalTicker: func() (<-chan time.Time, func()) { return make(chan time.Time), func() {} }}}
			if err := x.confirmPreview(preview, strings.Contains(preview, "delete")); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if !r.closed {
				t.Fatal("reader not closed")
			}
		})
	}
}
func TestDestructivePhraseCancellableReader(t *testing.T) {
	for _, tc := range []struct {
		input string
		ok    bool
	}{{"apply-destructive\n", true}, {"apply\n", false}, {"\x1b", false}, {"", false}} {
		r := &approvalRead{Reader: strings.NewReader(tc.input)}
		x := execution{ctx: context.Background(), approved: true, o: Options{Output: io.Discard, ApprovalReader: func() (cancelreader.CancelReader, error) { return r, nil }}}
		if err := x.confirmPreview("1 replace", true); (err == nil) != tc.ok {
			t.Fatalf("%q: %v", tc.input, err)
		}
	}
}
func TestReconfigureExplicitFlagsAndProductionCapacity(t *testing.T) {
	var prompts strings.Builder
	o := Options{Root: t.TempDir(), Output: &prompts, reader: bufio.NewReader(strings.NewReader(strings.Repeat("\n", 20))), Args: []string{"--operation", "reconfigure", "--scope", "backend", "--environment", "prod", "--account", "123456789012", "--profile", "", "--capture-reserved-concurrency", "10", "--capture-worker-max-concurrency", "5"}}
	_, c, _, err := parse(o)
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"Operation (", "AWS account", "AWS profile", "Production capture"} {
		if strings.Contains(prompts.String(), label) {
			t.Fatalf("explicit field prompted: %s", prompts.String())
		}
	}
	if c.Operation != "reconfigure" || c.Account != "123456789012" || c.Profile != "" || c.CaptureReserved != 10 || c.CaptureWorker != 5 {
		t.Fatalf("flags overwritten %+v", c)
	}
}
func TestSavedProductionCapacityAndSharedSkip(t *testing.T) {
	for _, scope := range []string{"backend", "shared"} {
		t.Run(scope, func(t *testing.T) {
			root := t.TempDir()
			c := defaults()
			c.Environment = "prod"
			c.Scope = scope
			c.Account = "123456789012"
			c.StateBackend = "s3://state"
			c.CaptureReserved = 10
			c.CaptureWorker = 5
			s := saved{Version: 1}
			s.remember(c)
			if err := save(filepath.Join(root, ".attestra", "wizard.json"), s); err != nil {
				t.Fatal(err)
			}
			input := scope + "\n"
			if scope != "shared" {
				input += "prod\n"
			}
			input += "next\npreview\n"
			_, got, _, err := parse(Options{Root: root, Output: io.Discard, reader: bufio.NewReader(strings.NewReader(input))})
			if err != nil {
				t.Fatal(err)
			}
			if got.CaptureReserved != 10 || got.CaptureWorker != 5 {
				t.Fatalf("capacity changed %+v", got)
			}
		})
	}
}

func TestEnvironmentMenuSavedImportedAndCustom(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".attestra"), 0700)
	os.WriteFile(filepath.Join(home, ".attestra", "legacy-wizard.json"), []byte(`{"Version":1,"Environments":{"stage":{},"prod":{}}}`), 0600)
	root := t.TempDir()
	s := saved{Version: 1}
	c := defaults()
	c.Environment = "prod"
	s.remember(c)
	c.Environment = "dev"
	s.remember(c)
	if err := save(filepath.Join(root, ".attestra", "wizard.json"), s); err != nil {
		t.Fatal(err)
	}
	for _, pick := range []string{"2", "custom-env"} {
		var out bytes.Buffer
		_, got, _, err := parse(Options{Root: root, Output: &out, reader: bufio.NewReader(strings.NewReader("backend\n" + pick + "\nnext\nbuild\n" + strings.Repeat("\n", 30)))})
		if err != nil {
			t.Fatal(err)
		}
		want := "prod"
		if pick == "custom-env" {
			want = pick
		}
		if got.Environment != want || !strings.Contains(out.String(), "1) dev, 2) prod, 3) stage") || !strings.Contains(out.String(), "custom") {
			t.Fatalf("menu/selection: %+v %s", got, out.String())
		}
	}
}
