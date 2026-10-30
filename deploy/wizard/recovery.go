package wizard

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Stack selection does not recover ignored Pulumi YAML files. Restore the exact
// encrypted configuration and secrets-provider metadata through Pulumi itself.
func (x *execution) restoreConfig() error {
	raw, e := x.run("pulumi", "stack", "export", "--stack", x.stack)
	if e != nil {
		return fmt.Errorf("cannot determine authoritative configuration for %s; refusing to replace secrets: %w", x.stack, e)
	}
	var state struct {
		Deployment *struct{ Resources []json.RawMessage }
	}
	if json.Unmarshal(raw, &state) != nil {
		return errors.New("invalid stack state while recovering configuration")
	}
	deployed := state.Deployment != nil && len(state.Deployment.Resources) > 0
	if deployed {
		if _, e = x.run("pulumi", "config", "refresh", "--force", "--stack", x.stack, "--non-interactive"); e != nil {
			return fmt.Errorf("recover encrypted configuration with pulumi config refresh before continuing: %w", e)
		}
	} else {
		info, e := os.Stat(filepath.Join(x.dir, "Pulumi."+x.stack+".yaml"))
		if e != nil || info.Size() == 0 {
			return fmt.Errorf("existing stack %s has no authoritative deployed configuration and no valid local config; recover the original checkout/config or inspect and explicitly remove the unused stack before retrying; proofKey will not be regenerated", x.stack)
		}
	}
	config, e := x.config()
	if e != nil {
		return e
	}
	if e = x.checkPins(config, false); e != nil {
		return e
	}
	for _, key := range []string{"deploymentPrefix", "expectedAccountId", "expectedRegion"} {
		if _, ok := config[x.project+":"+key]; !ok {
			return fmt.Errorf("recovered/local stack config missing target binding %s; refusing to replace secrets", key)
		}
	}
	if deployed && x.project == "attestra-backend" {
		if proof, ok := config[x.project+":proofKey"]; !ok || !proof.Secret {
			return errors.New("authoritative deployed backend proofKey is missing or unencrypted; recover the original encrypted config before continuing")
		}
	}
	return nil
}
func (x *execution) checkPreviewConfig(shared bool, out map[string]any, existing map[string]configValue) error {
	values, e := x.desiredConfig(shared, out)
	if e != nil {
		return e
	}
	different := []string{}
	for key, value := range values {
		if shared && key == "sourceHash" {
			continue
		} // Export-only provenance does not drive resources.
		old, ok := existing[x.project+":"+key]
		if !ok || old.Secret || old.Value != value {
			different = append(different, key)
		}
	}
	// Explicit certificate choices also change desired infrastructure.
	certs := map[string]string{"apiCertificateArn": x.c.APICertificateARN, "appCertificateArn": x.c.AppCertificateARN}
	if shared {
		certs = map[string]string{"indexCertificateArn": x.c.IndexCertificateARN, "adminCertificateArn": x.c.AdminCertificateARN}
	}
	for key, value := range certs {
		if value != "" && existing[x.project+":"+key].Value != value {
			different = append(different, key)
		}
	}
	if len(different) > 0 {
		sort.Strings(different)
		return fmt.Errorf("read-only preview cannot represent pending configuration changes for %s (%s); no choices saved and no cloud configuration changed. Restore matching choices or use explicit --operation deploy --yes, which previews the approved desired configuration before applying", x.stack, strings.Join(different, ", "))
	}
	return nil
}

// The post-apply journal makes DNS/admin/export prerequisites resumable before
// public configuration advertises the custom hostname. It never repeats up.
func (x *execution) completeApplied(p *publication) error {
	if p.JournalVersion != 1 {
		return errors.New("pending receipt has no complete post-apply recovery journal; inspect the original deployment's DNS/admin/export before manual recovery")
	}
	if e := p.Desired.validate(); e != nil {
		return errors.New("invalid saved deployment choices in recovery journal")
	}
	if recoveryConfig(x.c) != recoveryConfig(p.Desired) {
		return errors.New("pending deployment uses different saved choices; resume the original choices before requesting another configuration")
	}

	if e := validateOutputs(x.c, p.Stack, p.Outputs); e != nil {
		return e
	}
	if e := validateShared(x.c, p.Shared); e != nil {
		return e
	}
	if p.URL != str(p.Shared, "apiUrl") {
		return errors.New("recovery journal registry endpoint does not match verified shared outputs")
	}
	work := *x
	work.c = p.Desired
	work.stack = p.Stack
	work.project = "attestra-backend"
	work.dir = filepath.Join(x.o.Root, "infra")
	if p.Stage != "postapply" && (!p.DNSDone || !p.AdminDone) {
		return errors.New("publication journal has unfinished DNS/admin prerequisites; inspect receipt before recovery")
	}
	if !p.DNSDone {
		if e := work.hostDNS(false, p.Outputs); e != nil {
			return fmt.Errorf("backend DNS completion pending; retry deploy or publish with the same saved target: %w", e)
		}
		p.DNSDone = true
		if e := x.storePublication(p); e != nil {
			return e
		}
	}
	if !p.AdminDone {
		if work.c.AdminEmail != "" {
			_, e := work.o.Runner.Run(work.ctx, work.o.Root, work.env, "go", "run", "./cmd/admin-setup", "-pool", str(p.Shared, "adminPoolId"), "-login", str(p.Shared, "adminLoginDomain"), "-table", str(p.Shared, "runtimeTableName"), "-email", work.c.AdminEmail, "-environment", p.Stack)
			if e != nil {
				return fmt.Errorf("backend admin completion pending; retry deploy or publish with the same saved target: %w", e)
			}
		}
		p.AdminDone = true
		if e := x.storePublication(p); e != nil {
			return e
		}
	}
	// Always recreate the public file in the current checkout, including ready or
	// complete receipt recovery from a different machine.
	if e := work.exportAndroid(p.Outputs, p.Shared); e != nil {
		return fmt.Errorf("Android export completion pending: %w", e)
	}
	if str(p.Outputs, "assetLinksUrl") == "" && work.c.AndroidAssociations == "" {
		fmt.Fprintln(work.o.Output, "Android app links/passkey association hosting is not configured; supply --android-associations for device association readiness.")
	}
	if p.Stage == "postapply" {
		return work.finishPublication(p, p.Outputs)
	}
	if p.Stage == "complete" {
		fmt.Fprintln(work.o.Output, "Deployment prerequisites and registry publication are complete.")
		return nil
	}
	return work.publish(p)
}

// Keep an explicit projection: new operational/setup fields must not prevent
// resuming an applied deployment, while resource and prerequisite changes do.
func recoveryConfig(c choices) choices {
	return choices{
		Prefix: c.Prefix, Environment: c.Environment, Account: c.Account, Region: c.Region,
		StateBackend: c.StateBackend, BaseDomain: c.BaseDomain,
		SenderDomain: c.SenderDomain, SenderAddress: c.SenderAddress, AdminEmail: c.AdminEmail,
		Route53ZoneID: c.Route53ZoneID, CloudflareZoneID: c.CloudflareZoneID,
		APIDomain: c.APIDomain, AppOrigin: c.AppOrigin, IndexDomain: c.IndexDomain, AdminDomain: c.AdminDomain,
		APICertificateARN: c.APICertificateARN, AppCertificateARN: c.AppCertificateARN,
		IndexCertificateARN: c.IndexCertificateARN, AdminCertificateARN: c.AdminCertificateARN,
		CaptureReserved: c.CaptureReserved, CaptureWorker: c.CaptureWorker, CaptureEnabled: c.CaptureEnabled,
		EnvironmentClass: c.EnvironmentClass, AndroidAssociations: c.AndroidAssociations,
	}
}
