package githubsetup

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ensureImmutablePolicy never widens the permissions of already-bound roles.
// Content changes produce a new ARN; same-name content drift fails closed.
func (c client) ensureImmutablePolicy(name, document string) (string, error) {
	arn := "arn:aws:iam::" + c.o.Account + ":policy/" + name
	var list struct {
		Policies []struct{ Arn, DefaultVersionId string }
	}
	if e := c.aws(&list, "iam", "list-policies", "--scope", "Local"); e != nil {
		return "", e
	}
	for _, p := range list.Policies {
		if p.Arn == arn {
			var version struct{ PolicyVersion struct{ Document any } }
			if e := c.aws(&version, "iam", "get-policy-version", "--policy-arn", arn, "--version-id", p.DefaultVersionId); e != nil {
				return "", e
			}
			var want any
			json.Unmarshal([]byte(document), &want)
			if canonical(version.PolicyVersion.Document) != canonical(want) {
				return "", fmt.Errorf("immutable generated policy %s has unexpected contents; review manually", arn)
			}
			return arn, nil
		}
	}
	fmt.Fprintf(c.o.Output, "Creating approved content-addressed policy %s\n", arn)
	var response struct{ Policy struct{ Arn string } }
	if e := c.awsInput(&response, map[string]any{"PolicyName": name, "PolicyDocument": document}, "iam", "create-policy"); e != nil {
		return "", e
	}
	if response.Policy.Arn != arn {
		return "", errors.New("generated policy ARN mismatch")
	}
	return arn, nil
}
func (c client) applyGenerated(role string, p GeneratedPolicy) error {
	// A generated role is entirely managed. Extra grants are not silently retained
	// under an approval that covers only the generated documents.
	var inline struct{ PolicyNames []string }
	if e := c.aws(&inline, "iam", "list-role-policies", "--role-name", role); e != nil {
		return e
	}
	if len(inline.PolicyNames) != 0 {
		return errors.New("automatic deployment role has unexpected inline permissions; review manually")
	}
	var attached struct{ AttachedPolicies []struct{ PolicyArn string } }
	if e := c.aws(&attached, "iam", "list-attached-role-policies", "--role-name", role); e != nil {
		return e
	}
	prefix := "arn:aws:iam::" + c.o.Account + ":policy/attestra-deployment-"
	for _, old := range attached.AttachedPolicies {
		if !strings.HasPrefix(old.PolicyArn, prefix) {
			return errors.New("automatic deployment role has unmanaged attached permissions; review manually")
		}
	}
	boundaryName := strings.TrimPrefix(p.BoundaryARN, "arn:aws:iam::"+c.o.Account+":policy/")
	arn, e := c.ensureImmutablePolicy(boundaryName, p.Boundary)
	if e != nil {
		return e
	}
	if arn != p.BoundaryARN {
		return errors.New("runtime boundary mismatch")
	}
	if p.ResetFunction != "" {
		var functions struct {
			Functions []struct{ FunctionName, Role string }
		}
		if e := c.aws(&functions, "lambda", "list-functions"); e != nil {
			return e
		}
		for _, fn := range functions.Functions {
			if fn.FunctionName == p.ResetFunction {
				prefix := "arn:aws:iam::" + c.o.Account + ":role/"
				if !strings.HasPrefix(fn.Role, prefix+p.RuntimePrefix) {
					return errors.New("existing reset Lambda requires automatic runtime adoption through a local SSO deployment first; boundary created, rerun deploy with --github-setup=false, then retry setup-github")
				}
				var r struct {
					Role struct {
						PermissionsBoundary struct{ PermissionsBoundaryArn string }
					}
				}
				if e := c.aws(&r, "iam", "get-role", "--role-name", strings.TrimPrefix(fn.Role, prefix)); e != nil {
					return e
				}
				if r.Role.PermissionsBoundary.PermissionsBoundaryArn != p.BoundaryARN {
					return errors.New("existing reset Lambda boundary differs; deploy approved automatic runtime pins locally before GitHub setup")
				}
			}
		}
	}
	wanted := map[string]bool{}
	for _, part := range p.DeploymentParts {
		arn, e := c.ensureImmutablePolicy("attestra-deployment-"+digest([]byte(part))[:24], part)
		if e != nil {
			return e
		}
		wanted[arn] = true
	}
	for _, old := range attached.AttachedPolicies {
		if !wanted[old.PolicyArn] {
			if e := c.aws(nil, "iam", "detach-role-policy", "--role-name", role, "--policy-arn", old.PolicyArn); e != nil {
				return e
			}
		} else {
			delete(wanted, old.PolicyArn)
		}
	}
	for arn := range wanted {
		if e := c.aws(nil, "iam", "attach-role-policy", "--role-name", role, "--policy-arn", arn); e != nil {
			return e
		}
	}
	return nil
}
