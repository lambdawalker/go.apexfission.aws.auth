package githubsetup

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func (c client) aws(result any, args ...string) error {
	return c.run(result, "aws", append(args, "--region", c.o.Region, "--output", "json", "--no-cli-pager")...)
}
func (c client) awsInput(result any, body any, args ...string) error {
	b, e := json.Marshal(body)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp("", "attestra-iam-*.json")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return c.aws(result, append(args, "--cli-input-json", "file://"+f.Name())...)
}
func trust(repo, env, provider string) map[string]any {
	return map[string]any{"Version": "2012-10-17", "Statement": []any{map[string]any{"Effect": "Allow", "Principal": map[string]string{"Federated": provider}, "Action": "sts:AssumeRoleWithWebIdentity", "Condition": map[string]any{"StringEquals": map[string]string{"token.actions.githubusercontent.com:aud": "sts.amazonaws.com", "token.actions.githubusercontent.com:sub": "repo:" + repo + ":environment:" + env}}}}}
}
func canonical(v any) string { b, _ := json.Marshal(v); return string(b) }
func (c client) ensureRole(t Target) (string, error) {
	var caller struct{ Account string }
	if e := c.aws(&caller, "sts", "get-caller-identity"); e != nil {
		return "", e
	}
	if caller.Account != c.o.Account {
		return "", errors.New("GitHub OIDC setup AWS account mismatch")
	}
	provider := "arn:aws:iam::" + c.o.Account + ":oidc-provider/token.actions.githubusercontent.com"
	name := "attestra-deploy-" + t.Environment
	if t.RoleARN != "" {
		prefix := "arn:aws:iam::" + c.o.Account + ":role/"
		if !strings.HasPrefix(t.RoleARN, prefix) {
			return "", errors.New("deployment role ARN account mismatch")
		}
		name = strings.TrimPrefix(t.RoleARN, prefix)
	}
	if !regexp.MustCompile(`^[A-Za-z0-9+=,.@_-]{1,64}$`).MatchString(name) {
		return "", errors.New("deployment role name must fit IAM limits (role paths not supported)")
	}
	var roles struct {
		Roles []struct {
			RoleName, Arn            string
			AssumeRolePolicyDocument any
		}
	}
	if e := c.aws(&roles, "iam", "list-roles"); e != nil {
		return "", e
	}
	desired := trust(c.o.Repository, t.Environment, provider)
	for _, role := range roles.Roles {
		if role.RoleName != name {
			continue
		}
		if canonical(role.AssumeRolePolicyDocument) != canonical(desired) {
			return "", errors.New("existing deployment role trust differs from exact repository/environment OIDC trust; review it manually, no trust was overwritten")
		}
		if e := c.verifyProvider(provider); e != nil {
			return "", e
		}
		if c.o.PolicyFile != "" {
			if e := c.applyPolicy(name); e != nil {
				return "", e
			}
		}
		fmt.Fprintf(c.o.Output, "Reusing repository/environment-scoped OIDC role %s; existing permissions are preserved\n", role.Arn)
		return role.Arn, nil
	}
	if c.o.PolicyFile == "" {
		return "", errors.New("deployment role missing: provide --github-policy with a reviewed least-privilege IAM policy JSON, or --github-role-arn for an existing exact-scoped role; no AdministratorAccess is generated")
	}
	if _, e := readPolicy(c.o.PolicyFile); e != nil {
		return "", e
	}
	var providers struct {
		Providers []struct{ Arn string } `json:"OpenIDConnectProviderList"`
	}
	if e := c.aws(&providers, "iam", "list-open-id-connect-providers"); e != nil {
		return "", e
	}
	found := false
	for _, p := range providers.Providers {
		found = found || p.Arn == provider
	}
	if !found {
		fmt.Fprintf(c.o.Output, "Creating GitHub OIDC provider and role %s with subject repo:%s:environment:%s using reviewed policy %s\n", name, c.o.Repository, t.Environment, c.o.PolicyFile)
		if e := c.awsInput(nil, map[string]any{"Url": "https://token.actions.githubusercontent.com", "ClientIDList": []string{"sts.amazonaws.com"}}, "iam", "create-open-id-connect-provider"); e != nil {
			return "", e
		}
	}
	if e := c.verifyProvider(provider); e != nil {
		return "", e
	}
	fmt.Fprintf(c.o.Output, "Creating role %s with exact repository/environment OIDC trust and reviewed deployment policy\n", name)
	var response struct{ Role struct{ Arn string } }
	if e := c.awsInput(&response, map[string]any{"RoleName": name, "AssumeRolePolicyDocument": canonical(desired)}, "iam", "create-role"); e != nil {
		return "", e
	}
	if response.Role.Arn != "arn:aws:iam::"+c.o.Account+":role/"+name {
		return "", errors.New("created deployment role ARN mismatch")
	}
	if e := c.applyPolicy(name); e != nil {
		return "", e
	}
	return response.Role.Arn, nil
}
func (c client) verifyProvider(arn string) error {
	var p struct {
		URL     string   `json:"Url"`
		Clients []string `json:"ClientIDList"`
	}
	if e := c.aws(&p, "iam", "get-open-id-connect-provider", "--open-id-connect-provider-arn", arn); e != nil {
		return e
	}
	if p.URL != "token.actions.githubusercontent.com" && p.URL != "https://token.actions.githubusercontent.com" {
		return errors.New("unexpected GitHub OIDC issuer")
	}
	for _, v := range p.Clients {
		if v == "sts.amazonaws.com" {
			return nil
		}
	}
	return errors.New("existing GitHub OIDC provider needs sts.amazonaws.com audience; inspect it manually")
}
func readPolicy(path string) (string, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	var p struct {
		Version   string
		Statement []struct {
			Effect   string
			Action   any
			Resource any
		}
	}
	if json.Unmarshal(b, &p) != nil || p.Version != "2012-10-17" || len(p.Statement) == 0 {
		return "", errors.New("invalid reviewed deployment policy JSON")
	}
	for _, s := range p.Statement {
		if s.Effect != "Allow" {
			continue
		}
		acts := []string{}
		switch v := s.Action.(type) {
		case string:
			acts = append(acts, v)
		case []any:
			for _, a := range v {
				str, ok := a.(string)
				if !ok {
					return "", errors.New("invalid policy action")
				}
				acts = append(acts, str)
			}
		}
		if len(acts) == 0 {
			return "", errors.New("policy actions required; NotAction policies unsupported")
		}
		for _, a := range acts {
			if a == "*" || strings.EqualFold(a, "iam:*") {
				return "", errors.New("reviewed policy must not grant wildcard administrator actions")
			}
		}
	}
	return string(b), nil
}
func (c client) applyPolicy(name string) error {
	policy, e := readPolicy(c.o.PolicyFile)
	if e != nil {
		return e
	}
	var old struct {
		Document any `json:"PolicyDocument"`
	}
	var list struct {
		Names []string `json:"PolicyNames"`
	}
	if e := c.aws(&list, "iam", "list-role-policies", "--role-name", name); e != nil {
		return e
	}
	found := false
	for _, n := range list.Names {
		found = found || n == "attestra-deployment"
	}
	if found {
		if e := c.aws(&old, "iam", "get-role-policy", "--role-name", name, "--policy-name", "attestra-deployment"); e != nil {
			return e
		}
		var want any
		json.Unmarshal([]byte(policy), &want)
		if canonical(old.Document) == canonical(want) {
			return nil
		}
	}
	fmt.Fprintf(c.o.Output, "Applying reviewed inline policy to %s (other policies preserved)\n", name)
	return c.awsInput(nil, map[string]any{"RoleName": name, "PolicyName": "attestra-deployment", "PolicyDocument": policy}, "iam", "put-role-policy")
}
