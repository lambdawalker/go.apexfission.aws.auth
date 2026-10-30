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
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type choices struct {
	GitHubSetup, GitHubAsked                                                                       bool
	GitHubRepository, GitHubPolicy, GitHubSharedRoleARN, GitHubBackendRoleARN, GitHubSecretVersion string
	GitHubForce, GitHubRotate                                                                      bool

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
	BuildJobs                                                                      int
	ForceRebuild, CleanCache                                                       bool
	ReplaceCloudflareToken                                                         bool `json:"-"`
}
type binding struct{ Account, Region, StateBackend string }
type saved struct {
	Version      int
	Choices      choices
	Bindings     map[string]binding
	Environments map[string]choices
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
	if c.Operation != "preview" && c.Operation != "deploy" && c.Operation != "status" && c.Operation != "health" && c.Operation != "publish" && c.Operation != "build" && c.Operation != "reconfigure" && c.Operation != "rotate-secrets" && c.Operation != "setup-github" {
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
	local := map[string]json.RawMessage{}
	if len(b) > 0 {
		var document struct{ Choices map[string]json.RawMessage }
		if json.Unmarshal(b, &document) == nil {
			local = document.Choices
		}
	}
	// Resolve the selected target before applying its saved answers. Version 1
	// Choices remains the compatibility representation for CI consumers.
	selected := s.Choices
	for i, arg := range o.Args {
		arg = "--" + strings.TrimLeft(arg, "-")
		for _, name := range []string{"prefix", "environment", "scope"} {
			value := ""
			found := false
			if strings.HasPrefix(arg, "--"+name+"=") {
				value = strings.TrimPrefix(arg, "--"+name+"=")
				found = true
			} else if arg == "--"+name && i+1 < len(o.Args) {
				value = o.Args[i+1]
				found = true
			}
			if found {
				switch name {
				case "prefix":
					selected.Prefix = value
				case "environment":
					selected.Environment = value
				case "scope":
					selected.Scope = value
				}
			}
		}
	}
	if len(o.Args) == 0 {
		for _, v := range []struct {
			label string
			p     *string
		}{{"Scope (all/admin-index/backend)", &selected.Scope}, {"Logical environment", &selected.Environment}, {"Prefix (- selects final)", &selected.Prefix}} {
			if v.p == &selected.Environment && normalizeScope(selected.Scope) == "shared" {
				continue
			}
			var names []string
			if v.p == &selected.Environment {
				var err error
				names, err = availableEnvironments(s)
				if err != nil {
					return s, selected, false, err
				}
				entries := make([]string, len(names))
				for i, name := range names {
					entries[i] = fmt.Sprintf("%d) %s", i+1, name)
				}
				fmt.Fprintf(o.Output, "Available environments: %s; enter a number or custom name\n", strings.Join(entries, ", "))
			}
			fmt.Fprintf(o.Output, "%s [%s]: ", v.label, *v.p)
			line, err := o.reader.ReadString('\n')
			if err != nil {
				return s, selected, false, errors.New("interactive input ended; use flags for unattended operation")
			}
			if line = strings.TrimSpace(line); line != "" {
				if v.p == &selected.Environment {
					if n, err := strconv.Atoi(line); err == nil && n > 0 && n <= len(names) {
						line = names[n-1]
					}
				}
				*v.p = line
			}
		}
		if selected.Prefix == "-" {
			selected.Prefix = ""
		}
	}
	// A Version1 shared CI snapshot belongs to the prefix, independent of
	// the backend environment supplied by a workflow dispatch.
	sharedSnapshot := normalizeScope(selected.Scope) == "shared" && normalizeScope(s.Choices.Scope) == "shared" && selected.identity() == s.Choices.identity()
	if sharedSnapshot {
		selected.Environment = s.Choices.Environment
	}
	c := defaults()
	if selected.Prefix == s.Choices.Prefix && selected.Environment == s.Choices.Environment && normalizeScope(selected.Scope) == normalizeScope(s.Choices.Scope) {
		c = s.Choices
	} else {
		local = map[string]json.RawMessage{}
	}
	if prior, ok := s.Environments[selected.identity()+"/"+selected.Environment+"/"+normalizeScope(selected.Scope)]; ok {
		c = prior
		raw, _ := json.Marshal(prior)
		_ = json.Unmarshal(raw, &local)
	}
	if _, exact := s.Environments[selected.identity()+"/"+selected.Environment+"/"+normalizeScope(selected.Scope)]; !exact && (selected.identity() != s.Choices.identity() || selected.Environment != s.Choices.Environment || normalizeScope(selected.Scope) != normalizeScope(s.Choices.Scope)) {
		for _, scope := range []string{"both", "backend", "shared"} {
			prior, ok := s.Environments[selected.identity()+"/"+selected.Environment+"/"+scope]
			if !ok && selected.identity() == s.Choices.identity() && selected.Environment == s.Choices.Environment && normalizeScope(s.Choices.Scope) == scope {
				prior = s.Choices
				ok = true
			}
			if ok {
				local = commonScopeAnswers(prior)
				raw, _ := json.Marshal(local)
				_ = json.Unmarshal(raw, &c)
				break
			}
		}
	}
	c.Prefix, c.Environment, c.Scope = selected.Prefix, selected.Environment, selected.Scope
	c.Operation = "preview" // A previous deploy never authorizes the next invocation.
	f := flag.NewFlagSet("deploy", flag.ContinueOnError)
	f.SetOutput(o.Output)
	for _, v := range []struct {
		name string
		p    *string
	}{{"prefix", &c.Prefix}, {"environment", &c.Environment}, {"scope", &c.Scope}, {"operation", &c.Operation}, {"account", &c.Account}, {"region", &c.Region}, {"state-backend", &c.StateBackend}, {"base-domain", &c.BaseDomain}, {"sender-domain", &c.SenderDomain}, {"sender-address", &c.SenderAddress}, {"admin-email", &c.AdminEmail}, {"profile", &c.Profile}, {"route53-zone-id", &c.Route53ZoneID}, {"cloudflare-zone-id", &c.CloudflareZoneID}, {"api-domain", &c.APIDomain}, {"app-origin", &c.AppOrigin}, {"index-domain", &c.IndexDomain}, {"admin-domain", &c.AdminDomain}, {"api-certificate-arn", &c.APICertificateARN}, {"app-certificate-arn", &c.AppCertificateARN}, {"index-certificate-arn", &c.IndexCertificateARN}, {"admin-certificate-arn", &c.AdminCertificateARN}, {"environment-class", &c.EnvironmentClass}, {"android-associations", &c.AndroidAssociations}} {
		if _, ok := local[fieldName(v.name)]; !ok {
			if value, exists := os.LookupEnv("ATTESTRA_" + strings.ToUpper(strings.ReplaceAll(v.name, "-", "_"))); exists {
				*v.p = value
			}
		}
		f.StringVar(v.p, v.name, *v.p, v.name)
	}
	f.StringVar(&c.GitHubRepository, "github-repository", c.GitHubRepository, "GitHub owner/repository")
	f.StringVar(&c.GitHubPolicy, "github-policy", c.GitHubPolicy, "reviewed deployment role IAM policy JSON")
	f.StringVar(&c.GitHubSharedRoleARN, "github-shared-role-arn", c.GitHubSharedRoleARN, "existing exact-scoped shared deployment role")
	f.StringVar(&c.GitHubBackendRoleARN, "github-backend-role-arn", c.GitHubBackendRoleARN, "existing exact-scoped selected backend deployment role")
	f.StringVar(&c.GitHubSecretVersion, "github-secret-version", c.GitHubSecretVersion, "nonsecret secret rotation identifier")
	f.BoolVar(&c.GitHubSetup, "github-setup", c.GitHubSetup, "configure GitHub deployment support before deploy")
	f.BoolVar(&c.GitHubForce, "github-force-reconcile", false, "redeliver managed GitHub configuration")
	f.BoolVar(&c.GitHubRotate, "github-rotate-secrets", false, "redeliver GitHub deployment secrets")
	reconfigure := false
	f.BoolVar(&reconfigure, "reconfigure", false, "prompt again for saved settings")
	f.BoolVar(&c.ReplaceCloudflareToken, "replace-cloudflare-token", false, "replace encrypted DNS token interactively")
	f.IntVar(&c.BuildJobs, "build-jobs", c.BuildJobs, "maximum parallel Lambda builds")
	f.BoolVar(&c.ForceRebuild, "force-rebuild", false, "ignore cached Lambda artifacts")
	f.BoolVar(&c.CleanCache, "clean-cache", false, "clear Lambda cache before build")
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
	c.Scope = normalizeScope(c.Scope)
	if sharedSnapshot {
		c.Environment = selected.Environment
	}
	explicit := map[string]bool{}
	f.Visit(func(v *flag.Flag) {
		explicit[v.Name] = true
		if v.Name == "github-setup" {
			c.GitHubAsked = true
		}
	})
	f.VisitAll(func(v *flag.Flag) {
		if _, localValue := local[fieldName(v.Name)]; !localValue {
			if _, exists := os.LookupEnv("ATTESTRA_" + strings.ToUpper(strings.ReplaceAll(v.Name, "-", "_"))); exists {
				explicit[v.Name] = true
			}
		}
	})
	// Scope and environment have already been selected, so import once before
	// any explicit-field short circuit can skip unrelated legacy preferences.
	skipped, err := importLegacy(&c, local, explicit)
	if err != nil {
		return s, c, yes, err
	}
	if len(o.Args) == 0 || reconfigure || c.Operation == "reconfigure" {
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
		reconfigure = reconfigure || c.Operation == "reconfigure"
		for _, v := range []struct {
			label string
			p     *string
		}{{"Prefix (- selects separate final identity)", &c.Prefix}, {"Logical environment", &c.Environment}, {"Scope (both/shared/backend)", &c.Scope}, {"Operation (preview/deploy/status/health/publish)", &c.Operation}, {"AWS account", &c.Account}, {"AWS region", &c.Region}, {"Existing S3 state base", &c.StateBackend}, {"Base domain", &c.BaseDomain}, {"Sender domain (externally verified SES)", &c.SenderDomain}, {"Sender email", &c.SenderAddress}, {"Bootstrap admin email (optional)", &c.AdminEmail}, {"AWS profile (existing CLI session, optional)", &c.Profile}, {"Route53 zone ID (optional; otherwise Cloudflare/preissued TLS)", &c.Route53ZoneID}, {"Android association JSON (optional; [] explicitly hosts empty associations)", &c.AndroidAssociations}, {"Environment class (optional test/production)", &c.EnvironmentClass}} {
			if v.p == &c.Prefix || v.p == &c.Environment || v.p == &c.Scope {
				continue
			}
			key := choiceStringField(&c, v.p)
			if explicit[flagName(key)] {
				continue
			}
			if c.Scope == "shared" && (v.p == &c.SenderDomain || v.p == &c.SenderAddress || v.p == &c.AdminEmail || v.p == &c.AndroidAssociations || v.p == &c.EnvironmentClass) {
				continue
			}
			if !reconfigure && v.p != &c.Prefix && v.p != &c.Environment && v.p != &c.Scope && v.p != &c.Operation {

				if !validSavedString(key, *v.p) {
					skipped[v.p] = false
				}
				if raw, ok := local[key]; ok {
					var text string
					if json.Unmarshal(raw, &text) == nil && text == *v.p && validSavedString(key, text) {
						skipped[v.p] = true
					}
				}

			}
			if skipped[v.p] && !reconfigure {
				continue
			}
			if e = prompt(v.label, v.p); e != nil {
				return s, c, yes, e
			}
		}
		if c.Scope != "shared" && (c.Environment == "prod" || c.Environment == "production" || c.EnvironmentClass == "production") {
			reserved := strconv.Itoa(c.CaptureReserved)
			worker := strconv.Itoa(c.CaptureWorker)
			if !explicit["capture-reserved-concurrency"] && (reconfigure || local["CaptureReserved"] == nil || c.CaptureReserved < 1) {
				if e = prompt("Production capture reserved concurrency per function (positive)", &reserved); e != nil {
					return s, c, yes, e
				}
			}
			if !explicit["capture-worker-max-concurrency"] && (reconfigure || local["CaptureWorker"] == nil || c.CaptureWorker < 2 || c.CaptureWorker > 1000 || c.CaptureReserved > 0 && c.CaptureWorker > c.CaptureReserved) {
				if e = prompt("Production capture worker maximum (2–1000)", &worker); e != nil {
					return s, c, yes, e
				}
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

func normalizeScope(s string) string {
	switch s {
	case "all":
		return "both"
	case "admin-index":
		return "shared"
	}
	return s
}
func (s *saved) remember(c choices) {
	c.ForceRebuild = false
	c.GitHubForce = false
	c.GitHubRotate = false
	c.CleanCache = false
	s.Choices = c
	if s.Environments == nil {
		s.Environments = map[string]choices{}
	}
	s.Environments[c.identity()+"/"+c.Environment+"/"+c.Scope] = c
}

func fieldName(flag string) string {
	names := map[string]string{"api": "API", "arn": "ARN", "id": "ID", "aws": "AWS"}
	out := ""
	for _, part := range strings.Split(flag, "-") {
		if name, ok := names[part]; ok {
			out += name
		} else {
			out += strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return out
}

func validSavedString(key, value string) bool {
	switch key {
	case "Account":
		return accountPattern.MatchString(value)
	case "Region":
		return regionPattern.MatchString(value)
	case "StateBackend":
		u, e := url.Parse(value)
		return e == nil && u.Scheme == "s3" && u.Host != ""
	case "BaseDomain":
		return validHost(value)
	case "SenderDomain":
		return value == "" || validHost(value)
	case "SenderAddress", "AdminEmail":
		if value == "" {
			return true
		}
		m, e := mail.ParseAddress(value)
		return e == nil && m.Address == value
	case "AndroidAssociations":
		if value == "" {
			return true
		}
		var a []json.RawMessage
		return json.Unmarshal([]byte(value), &a) == nil
	case "EnvironmentClass":
		return value == "" || value == "test" || value == "production"
	}
	return true
}

func choiceStringField(c *choices, p *string) string {
	fields := reflect.ValueOf(c).Elem()
	typ := fields.Type()
	for i := 0; i < fields.NumField(); i++ {
		if fields.Field(i).Kind() == reflect.String && fields.Field(i).Addr().Interface() == p {
			return typ.Field(i).Name
		}
	}
	return ""
}
func flagName(field string) string {
	for _, name := range []string{"operation", "account", "region", "state-backend", "base-domain", "sender-domain", "sender-address", "admin-email", "profile", "route53-zone-id", "android-associations", "environment-class"} {
		if fieldName(name) == field {
			return name
		}
	}
	return ""
}

// Only common infrastructure preferences cross scopes. Missing backend fields
// remain unsaved so a shared-only run cannot suppress required sender prompts.
func commonScopeAnswers(c choices) map[string]json.RawMessage {
	raw, _ := json.Marshal(c)
	var all map[string]json.RawMessage
	_ = json.Unmarshal(raw, &all)
	out := map[string]json.RawMessage{}
	for _, key := range []string{"Account", "Region", "StateBackend", "BaseDomain", "Profile", "Route53ZoneID", "CloudflareZoneID", "IndexDomain", "AdminDomain", "IndexCertificateARN", "AdminCertificateARN"} {
		out[key] = all[key]
	}
	return out
}

// availableEnvironments merges saved and portable imported environment names.
func availableEnvironments(s saved) ([]string, error) {
	names := map[string]bool{}
	add := func(name string) {
		if envPattern.MatchString(name) && name != "shared" {
			names[name] = true
		}
	}
	add(s.Choices.Environment)
	for _, c := range s.Environments {
		add(c.Environment)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(home, ".attestra", "legacy-wizard.json"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		var imported struct {
			Version      int
			Environments map[string]json.RawMessage
		}
		if json.Unmarshal(data, &imported) != nil || imported.Version != 1 || imported.Environments == nil {
			return nil, errors.New("invalid legacy wizard export; re-run export-wizard")
		}
		for name := range imported.Environments {
			add(name)
		}
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}
