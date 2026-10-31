package githubsetup

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

type generatedIAMFake struct {
	roleName  string
	roleTrust any
	policies  map[string]string
	attached  map[string]bool
	writes    int
	inline    bool
	resetRole string
}

func (f *generatedIAMFake) Run(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
	value := func(flag string) string {
		for i, a := range args {
			if a == flag && i+1 < len(args) {
				return args[i+1]
			}
		}
		return ""
	}
	var out any = map[string]any{}
	switch strings.Join(args[:2], " ") {
	case "sts get-caller-identity":
		out = map[string]string{"Account": "123456789012"}
	case "iam list-roles":
		if f.roleName != "" {
			out = map[string]any{"Roles": []any{map[string]any{"RoleName": f.roleName, "Arn": "arn:aws:iam::123456789012:role/" + f.roleName, "AssumeRolePolicyDocument": f.roleTrust}}}
		}
	case "iam list-open-id-connect-providers":
		out = map[string]any{"OpenIDConnectProviderList": []any{map[string]string{"Arn": "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com"}}}
	case "iam get-open-id-connect-provider":
		out = map[string]any{"Url": "token.actions.githubusercontent.com", "ClientIDList": []string{"sts.amazonaws.com"}}
	case "iam create-role":
		b, e := os.ReadFile(strings.TrimPrefix(value("--cli-input-json"), "file://"))
		if e != nil {
			return nil, e
		}
		var v map[string]string
		json.Unmarshal(b, &v)
		f.roleName = v["RoleName"]
		json.Unmarshal([]byte(v["AssumeRolePolicyDocument"]), &f.roleTrust)
		f.writes++
		out = map[string]any{"Role": map[string]string{"Arn": "arn:aws:iam::123456789012:role/" + f.roleName}}
	case "iam list-role-policies":
		if f.inline {
			out = map[string]any{"PolicyNames": []string{"foreign"}}
		}
	case "iam list-attached-role-policies":
		a := []any{}
		for arn := range f.attached {
			a = append(a, map[string]string{"PolicyArn": arn})
		}
		out = map[string]any{"AttachedPolicies": a}
	case "iam list-policies":
		a := []any{}
		for arn := range f.policies {
			a = append(a, map[string]string{"Arn": arn, "DefaultVersionId": "v1"})
		}
		out = map[string]any{"Policies": a}
	case "iam get-policy-version":
		var doc any
		json.Unmarshal([]byte(f.policies[value("--policy-arn")]), &doc)
		out = map[string]any{"PolicyVersion": map[string]any{"Document": doc}}
	case "iam create-policy":
		b, e := os.ReadFile(strings.TrimPrefix(value("--cli-input-json"), "file://"))
		if e != nil {
			return nil, e
		}
		var v map[string]string
		json.Unmarshal(b, &v)
		arn := "arn:aws:iam::123456789012:policy/" + v["PolicyName"]
		f.policies[arn] = v["PolicyDocument"]
		f.writes++
		out = map[string]any{"Policy": map[string]string{"Arn": arn}}
	case "iam attach-role-policy":
		f.attached[value("--policy-arn")] = true
		f.writes++
	case "iam detach-role-policy":
		delete(f.attached, value("--policy-arn"))
		f.writes++
	case "lambda list-functions":
		if f.resetRole != "" {
			out = map[string]any{"Functions": []any{map[string]string{"FunctionName": "attestra-backend-reset-next-dev", "Role": f.resetRole}}}
		}
	default:
		return nil, errors.New("unexpected IAM action " + strings.Join(args, " "))
	}
	return json.Marshal(out)
}
func (f *generatedIAMFake) RunInput(context.Context, string, []string, io.Reader, string, ...string) ([]byte, error) {
	return nil, errors.New("unexpected stdin")
}
func TestGeneratedIAMReconciliationAndBoundaryDrift(t *testing.T) {
	p, e := Generate(policyFixture())
	if e != nil {
		t.Fatal(e)
	}
	f := &generatedIAMFake{policies: map[string]string{}, attached: map[string]bool{}}
	c := client{ctx: context.Background(), o: Options{Account: "123456789012", Region: "us-east-2", Runner: f, Output: io.Discard}, stdin: f}
	if e = c.applyGenerated("attestra-auto-next-dev", p); e != nil {
		t.Fatal(e)
	}
	if len(f.attached) != len(p.DeploymentParts) || len(f.policies) != len(p.DeploymentParts)+1 {
		t.Fatal("generated policies not attached")
	}
	f.writes = 0
	if e = c.applyGenerated("attestra-auto-next-dev", p); e != nil {
		t.Fatal(e)
	}
	if f.writes != 0 {
		t.Fatal("unchanged setup mutated IAM")
	}
	f.policies[p.BoundaryARN] = `{"Version":"2012-10-17","Statement":[]}`
	if e = c.applyGenerated("attestra-auto-next-dev", p); e == nil || f.writes != 0 {
		t.Fatal("boundary drift overwritten")
	}
}
func TestGeneratedIAMRejectsUnmanagedPermissionsAndLegacyReset(t *testing.T) {
	p, _ := Generate(policyFixture())
	for _, kind := range []string{"inline", "attached", "legacy-reset"} {
		f := &generatedIAMFake{policies: map[string]string{}, attached: map[string]bool{}}
		switch kind {
		case "inline":
			f.inline = true
		case "attached":
			f.attached["arn:aws:iam::aws:policy/AdministratorAccess"] = true
		case "legacy-reset":
			f.resetRole = "arn:aws:iam::123456789012:role/legacy-reset"
		}
		c := client{ctx: context.Background(), o: Options{Account: "123456789012", Region: "us-east-2", Runner: f, Output: io.Discard}, stdin: f}
		if e := c.applyGenerated("attestra-auto-next-dev", p); e == nil {
			t.Fatalf("accepted %s", kind)
		}
		if len(f.attached) > 0 && kind != "attached" {
			t.Fatal("granted access before validating existing reset")
		}
	}
}

func TestFreshAutomaticEnrollmentUsesGeneratedPolicies(t *testing.T) {
	p, e := Generate(policyFixture())
	if e != nil {
		t.Fatal(e)
	}
	f := &generatedIAMFake{policies: map[string]string{}, attached: map[string]bool{}}
	c := client{ctx: context.Background(), o: Options{Account: "123456789012", Region: "us-east-2", Repository: "owner/repo", Runner: f, Output: io.Discard}, stdin: f}
	target := Target{Environment: "next-dev", Generated: &p}
	arn, e := c.ensureRole(target)
	if e != nil {
		t.Fatal(e)
	}
	if arn != "arn:aws:iam::123456789012:role/attestra-auto-next-dev" || len(f.attached) != len(p.DeploymentParts) {
		t.Fatal("fresh role did not receive approved generated policies")
	}
	f.writes = 0
	if _, e = c.ensureRole(target); e != nil {
		t.Fatal(e)
	}
	if f.writes != 0 {
		t.Fatal("fresh enrollment retry changed IAM")
	}
}
