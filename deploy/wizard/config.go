// Package wizard orchestrates isolated Attestra deployments from consumer entrypoints.
package wizard

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type choices struct {
	Prefix, Environment, Scope, Operation                                          string
	Account, Region, StateBackend, BaseDomain                                      string
	SenderDomain, SenderAddress, AdminEmail                                        string
	Profile, Route53ZoneID, CloudflareZoneID                                       string
	APIDomain, AppOrigin, IndexDomain, AdminDomain                                 string
	APICertificateARN, AppCertificateARN, IndexCertificateARN, AdminCertificateARN string
	CaptureReserved, CaptureWorker                                                 int
	EnvironmentClass                                                               string
	CaptureEnabled                                                                 bool
	AndroidAssociations                                                            string
}
type binding struct{ Account, Region, StateBackend string }
type saved struct {
	Version  int
	Choices  choices
	Bindings map[string]binding
}

func defaults() choices {
	return choices{Prefix: "next", Environment: "dev", Scope: "both", Operation: "preview", Region: "us-east-2", BaseDomain: "attestra.com", CaptureReserved: -1, CaptureWorker: 2}
}
func (c choices) identity() string {
	if c.Prefix == "" {
		return "final"
	}
	return c.Prefix
}
func (c choices) stack(shared bool) string {
	if shared {
		return c.identity() + "-shared"
	}
	return c.identity() + "-" + c.Environment
}
func (c choices) host(label string) string {
	prefix := ""
	if c.Prefix != "" {
		prefix = c.Prefix + "."
	}
	return prefix + label + "." + c.BaseDomain
}
func (c choices) backend() string {
	return strings.TrimRight(c.StateBackend, "/") + "/attestra/" + c.identity()
}
func (c choices) domains() (index, admin, api, origin string) {
	index = c.IndexDomain
	if index == "" {
		index = c.host("index")
	}
	admin = c.AdminDomain
	if admin == "" {
		admin = c.host("admin")
	}
	api = c.APIDomain
	if api == "" {
		api = c.host("api." + c.Environment)
	}
	origin = c.AppOrigin
	if origin == "" {
		origin = "https://" + c.host(c.Environment)
	}
	return
}

var prefixPattern = regexp.MustCompile(`^[a-z][a-z0-9]{0,19}$`)
var envPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
var accountPattern = regexp.MustCompile(`^[0-9]{12}$`)
var regionPattern = regexp.MustCompile(`^[a-z]{2}(?:-[a-z]+)+-[0-9]+$`)

func validHost(s string) bool {
	if len(s) > 253 || !strings.Contains(s, ".") {
		return false
	}
	for _, part := range strings.Split(s, ".") {
		if len(part) > 63 || !regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`).MatchString(part) {
			return false
		}
	}
	return true
}
func (c choices) validate() error {
	if c.Prefix == "final" || c.Prefix != "" && !prefixPattern.MatchString(c.Prefix) {
		return errors.New("prefix must be empty or lowercase alphanumeric starting with a letter (maximum 20); final is reserved")
	}
	if !envPattern.MatchString(c.Environment) || c.Environment == "shared" || len(c.stack(false)) > 32 {
		return errors.New("environment must be DNS-safe, not shared, with full stack name at most 32 characters")
	}
	if c.Scope != "both" && c.Scope != "shared" && c.Scope != "backend" {
		return errors.New("scope must be both, shared, or backend")
	}
	if c.Operation != "preview" && c.Operation != "deploy" && c.Operation != "status" && c.Operation != "health" && c.Operation != "publish" {
		return errors.New("operation must be preview, deploy, status, health, or publish")
	}
	if !accountPattern.MatchString(c.Account) || !regionPattern.MatchString(c.Region) {
		return errors.New("12-digit account and AWS region are required")
	}
	u, e := url.Parse(c.StateBackend)
	if e != nil || u.Scheme != "s3" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Contains(u.Path, "..") || strings.Contains(u.Path, "/attestra/") {
		return errors.New("state-backend must be a dedicated s3://bucket/base path, without an identity suffix")
	}
	if !validHost(c.BaseDomain) {
		return errors.New("invalid base domain")
	}
	index, admin, api, origin := c.domains()
	for _, h := range []string{index, admin, api} {
		if !validHost(h) {
			return fmt.Errorf("invalid hostname %q", h)
		}
	}
	o, e := url.Parse(origin)
	if e != nil || o.Scheme != "https" || !validHost(o.Hostname()) || o.Host != o.Hostname() || o.User != nil || o.RawQuery != "" || o.Fragment != "" || o.Path != "" {
		return errors.New("app-origin must be an HTTPS origin without path, port, credentials, or query")
	}
	if c.Prefix != "" {
		for _, h := range []string{index, admin, api, o.Hostname()} {
			if !strings.HasPrefix(h, c.Prefix+".") {
				return fmt.Errorf("hostname %s must start with the selected prefix %s.", h, c.Prefix)
			}
			for _, legacy := range []string{"index." + c.BaseDomain, "admin." + c.BaseDomain, "api." + c.Environment + "." + c.BaseDomain, c.Environment + "." + c.BaseDomain} {
				if h == legacy {
					return fmt.Errorf("nonempty prefix cannot use legacy host %s", h)
				}
			}
		}
	}
	seen := map[string]bool{}
	for _, h := range []string{index, admin, api, o.Hostname()} {
		if seen[h] {
			return errors.New("index, admin, API, and app origin must have distinct hostnames")
		}
		seen[h] = true
	}
	for _, email := range []string{c.SenderAddress, c.AdminEmail} {
		if email != "" {
			m, e := mail.ParseAddress(email)
			if e != nil || m.Address != email {
				return errors.New("email addresses must be plain valid addresses")
			}
		}
	}
	if c.SenderDomain != "" && !validHost(c.SenderDomain) {
		return errors.New("invalid sender domain")
	}
	if c.CaptureReserved == 0 || c.CaptureReserved < -1 || c.CaptureWorker < 2 || c.CaptureWorker > 1000 || c.CaptureReserved > 0 && c.CaptureWorker > c.CaptureReserved {
		return errors.New("capture reservation must be -1 or positive, worker 2–1000 and no larger than reservation")
	}
	if c.EnvironmentClass != "" && c.EnvironmentClass != "test" && c.EnvironmentClass != "production" {
		return errors.New("environment-class must be test or production")
	}
	if (c.Environment == "prod" || c.Environment == "production") && c.EnvironmentClass == "test" {
		return errors.New("prod cannot use environment-class test")
	}
	if c.AndroidAssociations != "" {
		var a []json.RawMessage
		if json.Unmarshal([]byte(c.AndroidAssociations), &a) != nil {
			return errors.New("android-associations must be a JSON array")
		}
	}
	return nil
}
func (s *saved) bind(c choices) error {
	if s.Bindings == nil {
		s.Bindings = map[string]binding{}
	}
	b := binding{c.Account, c.Region, strings.TrimRight(c.StateBackend, "/")}
	if old, ok := s.Bindings[c.identity()]; ok && old != b {
		return fmt.Errorf("identity %s is already bound to account/region/state backend; choose a new prefix", c.identity())
	}
	s.Bindings[c.identity()] = b
	return nil
}
func save(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".wizard-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func parse(o Options) (saved, choices, bool, error) {
	s := saved{Version: 1, Choices: defaults()}
	p := filepath.Join(o.Root, ".attestra", "wizard.json")
	b, e := os.ReadFile(p)
	if e == nil {
		if json.Unmarshal(b, &s) != nil || s.Version != 1 {
			return s, s.Choices, false, errors.New("unsupported or corrupt wizard.json")
		}
	} else if !os.IsNotExist(e) {
		return s, s.Choices, false, e
	}
	c := s.Choices
	c.Operation = "preview" // A previous deploy never authorizes the next invocation.
	f := flag.NewFlagSet("deploy", flag.ContinueOnError)
	f.SetOutput(o.Output)
	for _, v := range []struct {
		name string
		p    *string
	}{{"prefix", &c.Prefix}, {"environment", &c.Environment}, {"scope", &c.Scope}, {"operation", &c.Operation}, {"account", &c.Account}, {"region", &c.Region}, {"state-backend", &c.StateBackend}, {"base-domain", &c.BaseDomain}, {"sender-domain", &c.SenderDomain}, {"sender-address", &c.SenderAddress}, {"admin-email", &c.AdminEmail}, {"profile", &c.Profile}, {"route53-zone-id", &c.Route53ZoneID}, {"cloudflare-zone-id", &c.CloudflareZoneID}, {"api-domain", &c.APIDomain}, {"app-origin", &c.AppOrigin}, {"index-domain", &c.IndexDomain}, {"admin-domain", &c.AdminDomain}, {"api-certificate-arn", &c.APICertificateARN}, {"app-certificate-arn", &c.AppCertificateARN}, {"index-certificate-arn", &c.IndexCertificateARN}, {"admin-certificate-arn", &c.AdminCertificateARN}, {"environment-class", &c.EnvironmentClass}, {"android-associations", &c.AndroidAssociations}} {
		f.StringVar(v.p, v.name, *v.p, v.name)
	}
	yes := false
	f.BoolVar(&yes, "yes", false, "approve deploy after preview")
	f.IntVar(&c.CaptureReserved, "capture-reserved-concurrency", c.CaptureReserved, "reservation per capture function")
	f.IntVar(&c.CaptureWorker, "capture-worker-max-concurrency", c.CaptureWorker, "SQS worker maximum")
	f.BoolVar(&c.CaptureEnabled, "capture-enabled", c.CaptureEnabled, "initial capture default")
	if e = f.Parse(o.Args); e != nil {
		return s, c, yes, e
	}
	if f.NArg() != 0 {
		return s, c, yes, errors.New("unexpected positional arguments")
	}
	if len(o.Args) == 0 {
		r := o.reader
		prompt := func(label string, p *string) error {
			fmt.Fprintf(o.Output, "%s [%s]: ", label, *p)
			line, e := r.ReadString('\n')
			if e != nil && e != io.EOF {
				return e
			}
			if e == io.EOF && line == "" {
				return errors.New("interactive input ended; use flags for unattended operation")
			}
			line = strings.TrimSpace(line)
			if line != "" {
				*p = line
			}
			return nil
		}
		for _, v := range []struct {
			label string
			p     *string
		}{{"Prefix (- selects separate final identity)", &c.Prefix}, {"Logical environment", &c.Environment}, {"Scope (both/shared/backend)", &c.Scope}, {"Operation (preview/deploy/status/health/publish)", &c.Operation}, {"AWS account", &c.Account}, {"AWS region", &c.Region}, {"Existing S3 state base", &c.StateBackend}, {"Base domain", &c.BaseDomain}, {"Sender domain (externally verified SES)", &c.SenderDomain}, {"Sender email", &c.SenderAddress}, {"Bootstrap admin email (optional)", &c.AdminEmail}, {"AWS profile (existing CLI session, optional)", &c.Profile}, {"Route53 zone ID (optional; otherwise Cloudflare/preissued TLS)", &c.Route53ZoneID}, {"Android association JSON (optional; [] explicitly hosts empty associations)", &c.AndroidAssociations}, {"Environment class (optional test/production)", &c.EnvironmentClass}} {
			if e = prompt(v.label, v.p); e != nil {
				return s, c, yes, e
			}
		}
		if c.Environment == "prod" || c.Environment == "production" || c.EnvironmentClass == "production" {
			reserved := strconv.Itoa(c.CaptureReserved)
			worker := strconv.Itoa(c.CaptureWorker)
			if e = prompt("Production capture reserved concurrency per function (positive)", &reserved); e != nil {
				return s, c, yes, e
			}
			if e = prompt("Production capture worker maximum (2–1000)", &worker); e != nil {
				return s, c, yes, e
			}
			var conversion error
			c.CaptureReserved, conversion = strconv.Atoi(reserved)
			if conversion != nil {
				return s, c, yes, errors.New("invalid capture reservation")
			}
			c.CaptureWorker, conversion = strconv.Atoi(worker)
			if conversion != nil {
				return s, c, yes, errors.New("invalid capture worker maximum")
			}
		}
		if c.Prefix == "-" {
			c.Prefix = ""
		}
	}
	return s, c, yes, nil
}
