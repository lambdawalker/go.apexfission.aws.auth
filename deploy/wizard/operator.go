package wizard

import (
	"errors"
	"fmt"
	"github.com/charmbracelet/x/term"
	"github.com/lambdawalker/go.apexfission.aws.auth/deploy/vault"
	"github.com/muesli/cancelreader"
	"io"
	"os"
	"strings"
	"time"
)

func (x *execution) secrets(f *os.File, interactive bool) error {
	if !interactive {
		if x.c.ReplaceCloudflareToken {
			return errors.New("DNS token replacement requires an interactive terminal")
		}
		if x.c.Operation == "rotate-secrets" {
			return errors.New("secret rotation requires an interactive terminal")
		}
		if (x.c.Operation == "deploy" || x.c.Operation == "setup-github") && os.Getenv("PULUMI_CONFIG_PASSPHRASE") == "" && os.Getenv("PULUMI_CONFIG_PASSPHRASE_FILE") == "" {
			return errors.New("set PULUMI_CONFIG_PASSPHRASE for unattended deployment")
		}
		return nil
	}
	read := func(label string) (string, error) {
		fmt.Fprint(x.o.Output, label+" (hidden): ")
		b, e := term.ReadPassword(f.Fd())
		fmt.Fprintln(x.o.Output)
		s := string(b)
		clear(b)
		return s, e
	}
	path, scope, e := vault.Location(x.o.Root, x.c.identity())
	if e != nil {
		return e
	}
	_, e = os.Stat(path)
	exists := e == nil
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	phrase, e := read("Credential vault passphrase")
	if e != nil {
		return e
	}
	var v *vault.Vault
	var values vault.Credentials
	if exists {
		v, values, e = vault.Unlock(path, scope, phrase)
	} else {
		confirm, err := read("Confirm new vault passphrase")
		if err != nil {
			return err
		}
		if phrase != confirm {
			return errors.New("vault passphrases do not match")
		}
		v, e = vault.Create(path, scope, phrase)
	}
	if e != nil {
		return e
	}
	defer v.Close()
	x.secretMetadataKey = v.DeriveKey("github-delivery-v1")
	if x.c.Operation == "rotate-secrets" {
		next, e := read("New vault passphrase")
		if e != nil {
			return e
		}
		confirm, e := read("Confirm new vault passphrase")
		if e != nil {
			return e
		}
		if next != confirm {
			return errors.New("vault passphrases do not match")
		}
		rotated, e := vault.RotatePassphrase(v, values, next)
		if e == nil {
			rotated.Close()
		}
		return e
	}
	if os.Getenv("PULUMI_CONFIG_PASSPHRASE") == "" && os.Getenv("PULUMI_CONFIG_PASSPHRASE_FILE") == "" {
		if values.Pulumi == "" {
			values.Pulumi, e = read("Pulumi config passphrase (shared across prefix)")
			if e != nil {
				return e
			}
			confirm, err := read("Confirm Pulumi config passphrase")
			if err != nil {
				return err
			}
			if confirm != values.Pulumi || confirm == "" {
				return errors.New("Pulumi passphrases do not match or are empty")
			}
		}
		x.env = withEnv(x.env, "PULUMI_CONFIG_PASSPHRASE", values.Pulumi)
	}
	if x.c.ReplaceCloudflareToken {
		token, err := read("Replacement Cloudflare API token")
		if err != nil {
			return err
		}
		if token == "" {
			return errors.New("replacement Cloudflare token must not be empty")
		}
		values.Cloudflare = token
		if os.Getenv("CLOUDFLARE_API_TOKEN") == "" {
			x.cloudflareToken = token
		}
	}
	if x.cloudflareToken == "" {
		x.cloudflareToken = values.Cloudflare
		if x.cloudflareToken == "" && (x.c.Operation == "deploy" || x.c.Operation == "setup-github" || x.c.GitHubSetup) && x.c.Route53ZoneID == "" {
			x.cloudflareToken, e = read("Cloudflare API token (optional)")
			if e != nil {
				return e
			}
			values.Cloudflare = x.cloudflareToken
		}
	}
	if x.cloudflareToken != "" {
		x.env = withEnv(x.env, "CLOUDFLARE_API_TOKEN", x.cloudflareToken)
	}
	if phrase == values.Pulumi && values.Pulumi != "" {
		return errors.New("use different vault and Pulumi passphrases")
	}
	return v.Save(values)
}
func destructivePreview(s string) bool {
	s = strings.ToLower(s)
	return strings.Contains(s, "delete") || strings.Contains(s, "replace") || strings.Contains(s, "destroy")
}
func (x *execution) confirmPreview(preview string) error {
	destructive := destructivePreview(preview)
	if x.o.ConfirmPreview != nil {
		return x.o.ConfirmPreview(x.ctx, preview, destructive)
	}
	if x.approved && !destructive {
		return nil
	}

	var r cancelreader.CancelReader
	if x.o.ApprovalReader != nil {
		var err error
		r, err = x.o.ApprovalReader()
		if err != nil {
			return err
		}
	} else {
		f, ok := x.o.Input.(*os.File)
		if !ok || !term.IsTerminal(f.Fd()) || f.Fd() != os.Stdin.Fd() {
			return errors.New("reviewed deployment requires --yes; destructive changes require terminal confirmation")
		}
		state, err := term.MakeRaw(f.Fd())
		if err != nil {
			return err
		}
		defer term.Restore(f.Fd(), state)
		r, err = cancelreader.NewReader(f)
		if err != nil {
			return err
		}
	}
	defer r.Close()
	if destructive {
		fmt.Fprint(x.o.Output, "Destructive changes detected. Type apply-destructive to proceed: ")
		return x.readApproval(r, nil, true)
	}
	fmt.Fprintln(x.o.Output, "Preview complete. Applying in 5 seconds; Yes/Enter proceeds, No/Escape/Ctrl-C cancels.")
	var ticks <-chan time.Time
	var stop func()
	if x.o.ApprovalTicker != nil {
		ticks, stop = x.o.ApprovalTicker()
	} else {
		ticker := time.NewTicker(time.Second)
		ticks = ticker.C
		stop = ticker.Stop
	}
	defer stop()
	return x.readApproval(r, ticks, false)
}
func (x *execution) readApproval(r cancelreader.CancelReader, ticks <-chan time.Time, destructive bool) error {
	type input struct {
		key byte
		err error
	}
	events := make(chan input)
	finished := make(chan struct{})
	quit := make(chan struct{})
	go func() {
		defer close(finished)
		for {
			b := make([]byte, 1)
			n, err := r.Read(b)
			event := input{err: err}
			if n > 0 {
				event.key = b[0]
				event.err = nil
			} else if err == nil {
				event.err = io.EOF
			}
			select {
			case events <- event:
			case <-quit:
				return
			}
			if event.err != nil {
				return
			}
		}
	}()
	defer func() { close(quit); r.Cancel(); <-finished }()
	remaining := 5
	var phrase strings.Builder
	if !destructive {
		fmt.Fprintf(x.o.Output, "%d...\r\n", remaining)
	}
	for {
		if err := x.ctx.Err(); err != nil {
			return err
		}
		select {
		case <-x.ctx.Done():
			return x.ctx.Err()
		case event := <-events:
			if event.err != nil {
				return errors.New("deployment cancelled: input ended")
			}
			key := event.key
			if destructive {
				if key == 27 || key == 3 || key == 4 {
					return errors.New("destructive deployment cancelled")
				}
				if key == '\r' || key == '\n' {
					if phrase.String() == "apply-destructive" {
						return nil
					}
					return errors.New("destructive deployment cancelled")
				}
				if phrase.Len() >= 64 {
					return errors.New("destructive deployment cancelled")
				}
				phrase.WriteByte(key)
				fmt.Fprint(x.o.Output, string(key))
				continue
			}
			if key == 'y' || key == 'Y' || key == '\r' || key == '\n' {
				return nil
			}
			return errors.New("deployment cancelled")
		case <-ticks:
			remaining--
			if remaining == 0 {
				return nil
			}
			fmt.Fprintf(x.o.Output, "%d...\r\n", remaining)
		}
	}
}
