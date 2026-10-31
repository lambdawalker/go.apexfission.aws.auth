package githubsetup

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func policyFixture() PolicyInput {
	return PolicyInput{Repository: "owner/repo", Account: "123456789012", Region: "us-east-2", Stack: "next-dev", Scope: "backend", Identity: "next", StateBackend: "s3://state-bucket/attestra/next", SenderDomain: "dev.info.example.com", SenderAddress: "verify@dev.info.example.com", Domains: []string{"next.dev.example.com"}}
}
func TestGeneratedPoliciesBoundRolesAndState(t *testing.T) {
	in := policyFixture()
	p, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := Generate(in)
	if p.Digest != again.Digest || p.Deployment != again.Deployment {
		t.Fatal("unstable generation")
	}
	if len(p.DeploymentParts) > 8 || len(p.Boundary) > 6144 {
		t.Fatal("IAM quota exceeded")
	}
	for _, want := range []string{"iam:PermissionsBoundary", p.BoundaryARN, "iam:PassedToService", "lambda.amazonaws.com", "arn:aws:s3:::state-bucket/attestra/next/*"} {
		if !strings.Contains(p.Deployment, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(p.Deployment, `"iam:*"`) || strings.Contains(p.Deployment, `"iam:CreatePolicy"`) || strings.Contains(p.Deployment, "attestra-deploy-next-dev") {
		t.Fatal("deployment role can administer itself/boundary")
	}
	if strings.Contains(p.Boundary, "state-bucket") {
		t.Fatal("runtime can access state")
	}
	in.Scope = "shared"
	in.Stack = "next-shared"
	in.SenderDomain = ""
	shared, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(shared.Deployment, "id-evidence") || strings.Contains(shared.Deployment, "sqs:CreateQueue") {
		t.Fatal("shared policy includes capture infrastructure")
	}
	if shared.RuntimePrefix == p.RuntimePrefix || shared.Digest == p.Digest {
		t.Fatal("targets share IAM identity")
	}
	var document any
	if json.Unmarshal([]byte(p.Deployment), &document) != nil {
		t.Fatal("bad JSON")
	}
}
func TestGeneratedPoliciesRejectUnsafeInputs(t *testing.T) {
	for _, change := range []func(*PolicyInput){func(p *PolicyInput) { p.Account = "*" }, func(p *PolicyInput) { p.Region = "*" }, func(p *PolicyInput) { p.Scope = "all" }, func(p *PolicyInput) { p.Stack = "next-*" }, func(p *PolicyInput) { p.StateBackend = "s3://state-bucket/" }, func(p *PolicyInput) { p.KMSKeys = []string{"*"} }, func(p *PolicyInput) { p.Domains = []string{"*.example.com"} }} {
		in := policyFixture()
		change(&in)
		if _, err := Generate(in); err == nil {
			t.Fatalf("unsafe input accepted: %+v", in)
		}
	}
}
func TestGeneratedKMSRestrictedToS3AndContext(t *testing.T) {
	in := policyFixture()
	in.KMSKeys = []string{"arn:aws:kms:us-east-2:123456789012:key/11111111-1111-1111-1111-111111111111"}
	p, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"kms:ViaService", "s3.us-east-2.amazonaws.com", "kms:EncryptionContext:aws:s3:arn", "arn:aws:s3:::state-bucket", "arn:aws:s3:::state-bucket/attestra/next/*"} {
		if !strings.Contains(p.Deployment, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(p.Deployment, "kms:*") {
		t.Fatal("wildcard KMS")
	}
}

func TestGeneratedAndroidFitsIAMLimits(t *testing.T) {
	in := policyFixture()
	in.Android = true
	in.Route53ZoneID = "Z12345"
	in.KMSKeys = []string{"arn:aws:kms:us-east-2:123456789012:key/11111111-1111-1111-1111-111111111111"}
	p, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range p.DeploymentParts {
		if len(part) > 6144 {
			t.Fatal("oversize managed policy")
		}
	}
}

// A narrow evaluator tests the boundary conditions our generated grants use;
// it is not an AWS IAM simulator. Deny behavior is tested separately below.
func grants(p string, action, arn string, ctx map[string]string) bool {
	var doc policyDocument
	if json.Unmarshal([]byte(p), &doc) != nil {
		return false
	}
	match := func(pattern, value string) bool {
		return regexp.MustCompile("^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, ".*") + "$").MatchString(value)
	}
	for _, s := range doc.Statement {
		if s.Effect != "Allow" {
			continue
		}
		a, r := false, false
		for _, v := range s.Action {
			a = a || match(v, action)
		}
		for _, v := range s.Resource {
			r = r || match(v, arn)
		}
		if !a || !r {
			continue
		}
		ok := true
		for op, values := range s.Condition {
			for key, want := range values {
				actual, exists := ctx[key]
				if op == "StringLikeIfExists" && !exists {
					continue
				}
				alternatives := []string{}
				switch v := want.(type) {
				case string:
					alternatives = append(alternatives, v)
				case []any:
					for _, x := range v {
						alternatives = append(alternatives, x.(string))
					}
				}
				found := false
				for _, v := range alternatives {
					if op == "ArnLike" || op == "StringLike" || op == "StringLikeIfExists" {
						found = found || match(v, actual)
					} else {
						found = found || v == actual
					}
				}
				ok = ok && exists && found
			}
		}
		if ok {
			return true
		}
	}
	return false
}
func TestGeneratedGrantBoundaries(t *testing.T) {
	p, e := Generate(policyFixture())
	if e != nil {
		t.Fatal(e)
	}
	role := "arn:aws:iam::123456789012:role/" + p.RuntimePrefix + "capture-role"
	for _, tt := range []struct {
		action, arn string
		ctx         map[string]string
		want        bool
	}{
		{"iam:CreateRole", role, nil, false},
		{"iam:CreateRole", role, map[string]string{"iam:PermissionsBoundary": p.BoundaryARN}, true},
		{"iam:CreateRole", role, map[string]string{"iam:PermissionsBoundary": "arn:aws:iam::123456789012:policy/other"}, false},
		{"iam:PassRole", role, map[string]string{"iam:PassedToService": "lambda.amazonaws.com"}, true},
		{"iam:PassRole", role, map[string]string{"iam:PassedToService": "ec2.amazonaws.com"}, false},
		{"iam:PutRolePolicy", "arn:aws:iam::123456789012:role/attestra-auto-next-dev", nil, false},
		{"iam:CreatePolicy", "arn:aws:iam::123456789012:policy/any", nil, false},
		{"lambda:UpdateFunctionCode", "arn:aws:lambda:us-east-2:123456789012:function:" + p.RuntimePrefix + "capture", nil, true},
		{"lambda:UpdateFunctionCode", "arn:aws:lambda:us-east-2:123456789012:function:capture-legacy", nil, false},
		{"lambda:UpdateFunctionCode", "arn:aws:lambda:us-west-2:123456789012:function:" + p.RuntimePrefix + "capture", nil, false},
		{"s3:GetObject", "arn:aws:s3:::state-bucket/attestra/next/.pulumi/meta.yaml", nil, true},
		{"s3:GetObject", "arn:aws:s3:::state-bucket/attestra/final/.pulumi/meta.yaml", nil, false},
		{"s3:ListBucket", "arn:aws:s3:::state-bucket", nil, true},
		{"cognito-idp:CreateUserPool", "*", map[string]string{"aws:RequestedRegion": "us-east-2"}, true},
		{"cognito-idp:CreateUserPool", "*", map[string]string{"aws:RequestedRegion": "us-west-2"}, false},
	} {
		if got := grants(p.Deployment, tt.action, tt.arn, tt.ctx); got != tt.want {
			t.Errorf("%s %s = %v want %v", tt.action, tt.arn, got, tt.want)
		}
	}
	var boundary policyDocument
	json.Unmarshal([]byte(p.Boundary), &boundary)
	if boundary.Statement[0].Effect != "Deny" || boundary.Statement[0].Action[0] != "iam:*" {
		t.Fatal("runtime lacks explicit identity-administration deny")
	}
}

func TestApplicationBucketFamiliesCannotContainState(t *testing.T) {
	for _, bucket := range []string{"id-evidence-state", "app-associations-state"} {
		in := policyFixture()
		in.Android = true
		in.StateBackend = "s3://" + bucket + "/attestra/next"
		p, e := Generate(in)
		if e == nil {
			t.Errorf("accepted colliding state bucket %s", bucket)
		}
		if grants(p.Boundary, "s3:GetObject", "arn:aws:s3:::"+bucket+"/attestra/final/.pulumi/meta.yaml", nil) || grants(p.Deployment, "s3:PutBucketPolicy", "arn:aws:s3:::"+bucket, nil) {
			t.Fatal("automatic policy grants application permissions over state")
		}
	}
}
