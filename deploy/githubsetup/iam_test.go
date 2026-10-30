package githubsetup

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type iamFake struct {
	foreign bool
	trust   any
	writes  int
}

func (f *iamFake) Run(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	var out any
	switch strings.Join(args[:2], " ") {
	case "sts get-caller-identity":
		account := "123456789012"
		if f.foreign {
			account = "999999999999"
		}
		out = map[string]string{"Account": account}
	case "iam list-roles":
		out = map[string]any{"Roles": []any{map[string]any{"RoleName": "attestra-deploy-next-dev", "Arn": "arn:aws:iam::123456789012:role/attestra-deploy-next-dev", "AssumeRolePolicyDocument": f.trust}}}
	case "iam get-open-id-connect-provider":
		out = map[string]any{"Url": "token.actions.githubusercontent.com", "ClientIDList": []string{"sts.amazonaws.com"}}
	default:
		f.writes++
		return nil, errors.New("unexpected mutation")
	}
	return json.Marshal(out)
}
func (f *iamFake) RunInput(context.Context, string, []string, io.Reader, string, ...string) ([]byte, error) {
	return nil, errors.New("unexpected stdin")
}
func TestExistingOIDCRoleVerifiedWithoutMutation(t *testing.T) {
	const provider = "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com"
	f := &iamFake{trust: trust("owner/repo", "next-dev", provider)}
	c := client{context.Background(), Options{Account: "123456789012", Region: "us-east-2", Repository: "owner/repo", Runner: f, Output: io.Discard}, f}
	if _, err := c.ensureRole(Target{Environment: "next-dev"}); err != nil {
		t.Fatal(err)
	}
	if f.writes != 0 {
		t.Fatal("existing role mutated")
	}
	f.trust = trust("other/repo", "next-dev", provider)
	if _, err := c.ensureRole(Target{Environment: "next-dev"}); err == nil {
		t.Fatal("foreign trust accepted")
	}
	f.foreign = true
	if _, err := c.ensureRole(Target{Environment: "next-dev"}); err == nil {
		t.Fatal("foreign account accepted")
	}
}
func TestRejectAdministratorPolicy(t *testing.T) {
	p := filepath.Join(t.TempDir(), "policy.json")
	for _, action := range []string{"*", "iam:*"} {
		os.WriteFile(p, []byte(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"`+action+`","Resource":"*"}]}`), 0600)
		if _, e := readPolicy(p); e == nil {
			t.Fatal("admin policy accepted")
		}
	}
	os.WriteFile(p, []byte(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:GetObject","Resource":"arn:aws:s3:::state/attestra/next/*"}]}`), 0600)
	if _, e := readPolicy(p); e != nil {
		t.Fatal(e)
	}
}
