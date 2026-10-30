package wizard

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
)

type ssoRunner struct {
	calls        []string
	renewed      bool
	interactive  int
	noSSO        bool
	wrongAccount bool
}

func (r *ssoRunner) Run(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	call := strings.Join(args, " ")
	r.calls = append(r.calls, call)
	if strings.Contains(call, "sts get-caller-identity") {
		if !r.renewed {
			return nil, ErrSSOSessionExpired
		}
		if r.wrongAccount {
			return []byte(`{"Account":"999999999999"}`), nil
		}
		return []byte(`{"Account":"123456789012"}`), nil
	}
	if strings.Contains(call, "configure get") {
		if r.noSSO {
			return nil, errors.New("not configured")
		}
		return []byte("my-sso-session"), nil
	}
	return nil, errors.New("unexpected")
}
func (r *ssoRunner) RunInteractive(ctx context.Context, dir string, env []string, input io.Reader, output io.Writer, name string, args ...string) error {
	r.interactive++
	io.WriteString(output, "Open https://device.example and enter public-code\n")
	if err := ctx.Err(); err != nil {
		return err
	}
	r.renewed = true
	return nil
}
func TestSSORenewalStreamsAndReverifies(t *testing.T) {
	t.Setenv("CI", "")
	r := &ssoRunner{}
	var output strings.Builder
	x := execution{ctx: context.Background(), c: choices{Profile: "chosen", Account: "123456789012"}, o: Options{Runner: r, Output: &output}}
	if err := x.renewSSO(true); err != nil {
		t.Fatal(err)
	}
	if r.interactive != 1 || !strings.Contains(output.String(), "public-code") {
		t.Fatal("instructions not streamed")
	}
	if len(r.calls) != 4 {
		t.Fatalf("STS not reverified: %v", r.calls)
	}
}
func TestSSORenewalGuardsAndCancellation(t *testing.T) {
	for _, mode := range []string{"ci", "noninteractive", "not-sso", "cancelled", "wrong-account"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("CI", "")
			r := &ssoRunner{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			interactive := true
			switch mode {
			case "ci":
				t.Setenv("CI", "true")
			case "noninteractive":
				interactive = false
			case "not-sso":
				r.noSSO = true
			case "cancelled":
				cancel()
			case "wrong-account":
				r.wrongAccount = true
			}
			x := execution{ctx: ctx, c: choices{Profile: "chosen", Account: "123456789012"}, o: Options{Runner: r, Output: io.Discard}}
			err := x.renewSSO(interactive)
			if mode == "ci" || mode == "noninteractive" {
				if err != nil || len(r.calls) != 0 || r.interactive != 0 {
					t.Fatal("unattended login")
				}
			} else if err == nil {
				t.Fatal("failed renewal accepted")
			}
			if mode == "not-sso" && r.interactive != 0 {
				t.Fatal("non-SSO login")
			}
		})
	}
}
func TestInteractiveProcessStreamsWhileRunning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, writer := io.Pipe()
	defer reader.Close()
	input, keepOpen, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer keepOpen.Close()
	done := make(chan error, 1)
	go func() {
		done <- process{}.RunInteractive(ctx, t.TempDir(), os.Environ(), input, writer, "sh", "-c", "printf 'visible-now\\n'; read ignored")
		writer.Close()
	}()
	line, err := bufio.NewReader(reader).ReadString('\n')
	if err != nil || line != "visible-now\n" {
		t.Fatal(line, err)
	}
	select {
	case err := <-done:
		t.Fatalf("login ended before cancellation: %v", err)
	default:
	}
	cancel()
	<-done
}
