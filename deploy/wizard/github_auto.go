package wizard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/deploy/githubsetup"
	"net/url"
	"path/filepath"
	"strings"
)

func (x *execution) prepareAutomaticGitHub(interactive bool) error {
	if x.o.GitHubPolicyProfile != githubsetup.AttestraProfile {
		return errors.New("consumer has not enabled the automatic Attestra IAM profile")
	}
	index, admin, api, origin := x.c.domains()
	u, _ := url.Parse(origin)
	policies := map[string]*githubsetup.GeneratedPolicy{}
	changed := false
	var ordered []string
	c := x.c
	for _, shared := range []bool{true, false} {
		if shared && c.Scope == "backend" || !shared && c.Scope == "shared" {
			continue
		}
		scope, domains, sender, android := "backend", []string{api}, c.SenderDomain, c.AndroidAssociations != ""
		if android {
			domains = append(domains, u.Hostname())
		}
		if shared {
			scope, domains, sender, android = "shared", []string{index, admin}, "", false
		}
		p, e := githubsetup.Generate(githubsetup.PolicyInput{Repository: c.GitHubRepository, Account: c.Account, Region: c.Region, Identity: c.identity(), Stack: c.stack(shared), Scope: scope, StateBackend: c.backend(), SenderDomain: sender, SenderAddress: c.SenderAddress, Domains: domains, KMSKeys: x.stateKMSKeys, Route53ZoneID: c.Route53ZoneID, Android: android})
		if e != nil {
			return e
		}
		policies[c.stack(shared)] = &p
		ordered = append(ordered, p.Digest)
		previous := c.GitHubBackendApproval
		if shared {
			previous = c.GitHubSharedApproval
		}
		changed = changed || previous != p.Digest
	}
	h := sha256.Sum256([]byte(strings.Join(ordered, "\n")))
	approval := hex.EncodeToString(h[:])
	if changed {
		fmt.Fprintln(x.o.Output, "Review generated GitHub AWS permissions. These are deployment permissions, not administrator access. Runtime execution roles require an immutable permissions boundary; GitHub cannot edit its own role or that boundary.")
		fmt.Fprintln(x.o.Output, "Scope: selected state bucket/identity prefix, target-specific Lambda code/roles, account/region and named resource families. API Gateway/Cognito/ACM use broader regional IDs; existing table/bucket families can span modular environments. Optional CloudFront access is global. Adopting automatic mode may replace existing runtime roles and functions; review the Pulumi plan before deploying.")
		for _, shared := range []bool{true, false} {
			p := policies[c.stack(shared)]
			if p == nil {
				continue
			}
			for _, item := range []struct{ suffix, body string }{{"deployment", p.Deployment}, {"runtime-boundary", p.Boundary}} {
				path := filepath.Join(x.o.Root, ".attestra", "github-policies", c.stack(shared)+"-"+item.suffix+".json")
				if e := save(path, json.RawMessage(item.body)); e != nil {
					return e
				}
				var formatted any
				_ = json.Unmarshal([]byte(item.body), &formatted)
				b, _ := json.MarshalIndent(formatted, "", "  ")
				fmt.Fprintf(x.o.Output, "\n%s permissions: %s\n%s\n", c.stack(shared), path, b)
			}
		}
		fmt.Fprintf(x.o.Output, "Permission review SHA256: %s\n", approval)
		if !interactive {
			if !x.approved || x.c.GitHubApprovalToken != approval {
				return fmt.Errorf("new/changed IAM permissions need review; rerun with --yes --github-approve-policy %s after reviewing the generated JSON files", approval)
			}
		} else {
			fmt.Fprint(x.o.Output, "Approve these AWS permission documents and GitHub setup? [y/N]: ")
			line, e := x.o.reader.ReadString('\n')
			if e != nil || !strings.EqualFold(strings.TrimSpace(line), "yes") && !strings.EqualFold(strings.TrimSpace(line), "y") {
				return errors.New("AWS permission approval cancelled; no GitHub/IAM setup was performed")
			}
		}
	}
	for _, shared := range []bool{true, false} {
		p := policies[c.stack(shared)]
		if p == nil {
			continue
		}
		if shared {
			c.GitHubSharedApproval, c.GitHubSharedPrefix, c.GitHubSharedBoundary = p.Digest, p.RuntimePrefix, p.BoundaryARN
		} else {
			c.GitHubBackendApproval, c.GitHubBackendPrefix, c.GitHubBackendBoundary = p.Digest, p.RuntimePrefix, p.BoundaryARN
		}
	}
	x.c = c
	x.generated = policies
	return nil
}
