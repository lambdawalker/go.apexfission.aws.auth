package wizard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/lambdawalker/go.apexfission.aws.auth/deploy/dns"
)

func (x *execution) acm(result any, region string, args ...string) error {
	args = append(args, "--region", region, "--output", "json", "--no-cli-pager", "--cli-connect-timeout", "10", "--cli-read-timeout", "30")
	b, e := x.run("aws", args...)
	if e != nil {
		return e
	}
	if result != nil && json.Unmarshal(b, result) != nil {
		return errors.New("invalid ACM response")
	}
	return nil
}

type certificateDescription struct {
	Certificate struct {
		CertificateArn, DomainName, Status string
		SubjectAlternativeNames            []string
		DomainValidationOptions            []struct {
			ResourceRecord struct{ Name, Type, Value string }
		}
	}
}
type certificateReceipt struct{ Owner, Account, Region, Domain, ARN, RequestedAt string }

func covers(pattern, host string) bool {
	if pattern == host {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*.")
		return strings.HasSuffix(host, "."+suffix) && strings.Count(host, ".") == strings.Count(suffix, ".")+1
	}
	return false
}
func (x *execution) describeCertificate(arn, region, host string) (certificateDescription, error) {
	var desc certificateDescription
	prefix := "arn:aws:acm:" + region + ":" + x.c.Account + ":certificate/"
	if !strings.HasPrefix(arn, prefix) || len(arn) <= len(prefix) {
		return desc, errors.New("certificate ARN account/region mismatch")
	}
	if e := x.acm(&desc, region, "acm", "describe-certificate", "--certificate-arn", arn); e != nil {
		return desc, e
	}
	matched := covers(desc.Certificate.DomainName, host)
	for _, name := range desc.Certificate.SubjectAlternativeNames {
		matched = matched || covers(name, host)
	}
	if !matched {
		return desc, fmt.Errorf("certificate does not cover %s", host)
	}
	return desc, nil
}
func (x *execution) certificate(region, host, provided string) (string, error) {
	if provided != "" {
		desc, e := x.describeCertificate(provided, region, host)
		if e != nil {
			return "", e
		}
		if desc.Certificate.Status != "ISSUED" {
			return "", fmt.Errorf("certificate for %s must be ISSUED before deployment", host)
		}
		return provided, nil
	}
	if x.cloudflareToken == "" {
		return "", errors.New("external DNS requires CLOUDFLARE_API_TOKEN for ACM validation, or all preissued --*-certificate-arn flags; alternatively provide --route53-zone-id. Sender/email DNS is never modified")
	}
	hash := sha256.Sum256([]byte(x.c.identity() + "/" + x.c.Account + "/" + region + "/" + host))
	id := hex.EncodeToString(hash[:])
	u, _ := url.Parse(x.c.backend())
	bucket := u.Host
	key := strings.TrimPrefix(u.Path, "/") + "/certificates/" + id + ".json"
	path := x.receiptPath("certificate-" + id)
	var listing struct{ Contents []struct{ Key string } }
	if e := x.aws(&listing, "s3api", "list-objects-v2", "--bucket", bucket, "--prefix", key, "--expected-bucket-owner", x.c.Account); e != nil {
		return "", e
	}
	exists := false
	for _, v := range listing.Contents {
		exists = exists || v.Key == key
	}
	receipt := certificateReceipt{Owner: "attestra-wizard-v1/" + x.c.identity(), Account: x.c.Account, Region: region, Domain: host, RequestedAt: time.Now().UTC().Format(time.RFC3339)}
	if exists {
		if e := x.aws(nil, "s3api", "get-object", "--bucket", bucket, "--key", key, "--expected-bucket-owner", x.c.Account, path); e != nil {
			return "", e
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return "", e
		}
		var old certificateReceipt
		if json.Unmarshal(b, &old) != nil || old.Owner != receipt.Owner || old.Account != receipt.Account || old.Region != region || old.Domain != host {
			return "", errors.New("certificate ownership receipt mismatch")
		}
		receipt = old
	}
	store := func() error {
		if e := save(path, receipt); e != nil {
			return e
		}
		return x.aws(nil, "s3api", "put-object", "--bucket", bucket, "--key", key, "--expected-bucket-owner", x.c.Account, "--body", path)
	}
	if receipt.ARN == "" {
		if exists {
			when, e := time.Parse(time.RFC3339, receipt.RequestedAt)
			if e != nil || time.Since(when) > 50*time.Minute {
				return "", fmt.Errorf("pending ACM request for %s needs manual reconciliation: inspect ACM tags attestra-wizard-owner=%s and receipt s3://%s/%s; supply existing issued ARN rather than creating another certificate", host, receipt.Owner, bucket, key)
			}
		} else if e := store(); e != nil {
			return "", e
		}
		var result struct{ CertificateArn string }
		if e := x.acm(&result, region, "acm", "request-certificate", "--domain-name", host, "--validation-method", "DNS", "--idempotency-token", id[:32], "--tags", "Key=attestra-wizard-owner,Value="+receipt.Owner); e != nil {
			return "", e
		}
		if result.CertificateArn == "" {
			return "", errors.New("ACM returned no certificate ARN")
		}
		receipt.ARN = result.CertificateArn
		if e := store(); e != nil {
			return "", e
		}
	}
	deadline := time.Now().Add(15 * time.Minute)
	published := false
	for {
		desc, e := x.describeCertificate(receipt.ARN, region, host)
		if e != nil {
			return "", e
		}
		switch desc.Certificate.Status {
		case "ISSUED":
			return receipt.ARN, nil
		case "PENDING_VALIDATION":
		default:
			return "", fmt.Errorf("ACM certificate %s is %s; inspect it before retrying", receipt.ARN, desc.Certificate.Status)
		}
		records := []dns.Record{}
		for _, v := range desc.Certificate.DomainValidationOptions {
			r := v.ResourceRecord
			if r.Name != "" && r.Type == "CNAME" && r.Value != "" {
				records = append(records, dns.Record{Type: "CNAME", Name: dns.Name(r.Name), Content: dns.Name(r.Value), TTL: 60})
			}
		}
		if !published && len(records) > 0 {
			if e = x.publishDNS(records); e != nil {
				return "", e
			}
			published = true
			fmt.Fprintf(x.o.Output, "ACM DNS validation published for %s; waiting up to 15 minutes for issuance (%s).\n", host, receipt.ARN)
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("ACM issuance deadline reached for %s (%s); validation DNS retained. Check authoritative DNS, then rerun deploy; no duplicate certificate requested", host, receipt.ARN)
		}
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-x.ctx.Done():
			timer.Stop()
			return "", x.ctx.Err()
		case <-timer.C:
		}
	}
}
func (x *execution) certificates(shared bool, existing map[string]configValue) error {
	if x.c.Route53ZoneID != "" {
		return nil
	}
	index, admin, api, origin := x.c.domains()
	host := strings.TrimPrefix(origin, "https://")
	targets := []struct{ key, region, host, arn string }{{"apiCertificateArn", x.c.Region, api, x.c.APICertificateARN}, {"appCertificateArn", "us-east-1", host, x.c.AppCertificateARN}}
	if !shared && x.c.AndroidAssociations == "" && existing["attestra-backend:androidAssociations"].Value == "" {
		targets = targets[:1]
	}
	if shared {
		targets = []struct{ key, region, host, arn string }{{"indexCertificateArn", x.c.Region, index, x.c.IndexCertificateARN}, {"adminCertificateArn", x.c.Region, admin, x.c.AdminCertificateARN}}
	}
	for _, t := range targets {
		arn, e := x.certificate(t.region, t.host, t.arn)
		if e != nil {
			return e
		}
		if e = x.set(t.key, arn); e != nil {
			return e
		}
	}
	return nil
}
func (x *execution) publishDNS(records []dns.Record) error {
	if x.o.DNS != nil {
		return x.o.DNS(x.ctx, records)
	}
	client := dns.NewClient(x.cloudflareToken)
	groups := map[string][]dns.Record{}
	for _, r := range records {
		zones, e := client.Zones(r.Name)
		if e != nil {
			return e
		}
		zone := ""
		best := 0
		for _, z := range zones {
			if x.c.CloudflareZoneID != "" && z.ID != x.c.CloudflareZoneID {
				continue
			}
			if dns.WithinZone(r.Name, z.Name) && len(z.Name) > best {
				zone = z.ID
				best = len(z.Name)
			}
		}
		if zone == "" {
			return fmt.Errorf("no authorized Cloudflare zone for %s; token requires Zone Read and DNS Edit", r.Name)
		}
		groups[zone] = append(groups[zone], r)
	}
	// Plan every zone before any write; conflicts never overwrite existing records.
	for zone, rs := range groups {
		if _, e := client.Plan(zone, rs); e != nil {
			return e
		}
	}
	for zone, rs := range groups {
		plan, e := client.Plan(zone, rs)
		if e != nil {
			return e
		}
		if e = client.Apply(zone, plan); e != nil {
			return e
		}
	}
	return nil
}
func (x *execution) hostDNS(shared bool, out map[string]any) error {
	if x.c.Route53ZoneID != "" {
		return nil
	}
	index, admin, api, origin := x.c.domains()
	mapping := [][2]string{{api, str(out, "apiDomainTarget")}, {strings.TrimPrefix(origin, "https://"), str(out, "appDomainTarget")}}
	if !shared && str(out, "appDomainTarget") == "" && x.c.AndroidAssociations == "" {
		mapping = mapping[:1]
	}
	if shared {
		mapping = [][2]string{{index, str(out, "domainTarget")}, {admin, str(out, "adminDomainTarget")}}
	}
	records := []dns.Record{}
	for _, m := range mapping {
		if m[1] == "" {
			return fmt.Errorf("missing hostname DNS target output for %s", m[0])
		}
		records = append(records, dns.Record{Type: "CNAME", Name: m[0], Content: dns.Name(m[1]), TTL: 60})
	}
	if x.cloudflareToken == "" && x.o.DNS == nil {
		for _, r := range records {
			fmt.Fprintf(x.o.Output, "Required external DNS: %s CNAME %s (DNS only, unproxied)\n", r.Name, r.Content)
		}
		return errors.New("infrastructure updated, hostname DNS requires manual publication above; rerun deploy with CLOUDFLARE_API_TOKEN to reconcile")
	}
	return x.publishDNS(records)
}
