package guard

import (
	"errors"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"regexp"
)

func runtimePins(ctx *pulumi.Context) (string, string, error) {
	c := config.New(ctx, "")
	prefix, boundary := c.Get("deploymentRolePrefix"), c.Get("deploymentBoundaryArn")
	if prefix == "" && boundary == "" {
		return "", "", nil
	}
	if !regexp.MustCompile(`^[0-9]{12}$`).MatchString(c.Get("expectedAccountId")) || !regexp.MustCompile(`^af-[a-f0-9]{16}-$`).MatchString(prefix) || !regexp.MustCompile(`^arn:aws:iam::`+regexp.QuoteMeta(c.Get("expectedAccountId"))+`:policy/attestra-runtime-[a-f0-9]{24}$`).MatchString(boundary) {
		return "", "", errors.New("invalid automatic runtime prefix/boundary pins")
	}
	return prefix, boundary, nil
}

// Role preserves manual mode. Automatic mode uses a target-specific name and
// immutable boundary, so a deployment role cannot create an unbounded identity.
func Role(ctx *pulumi.Context, name string, args *iam.RoleArgs, opts ...pulumi.ResourceOption) (*iam.Role, error) {
	prefix, boundary, e := runtimePins(ctx)
	if e != nil {
		return nil, e
	}
	a := *args
	if prefix != "" {
		if len(prefix+name) > 64 {
			return nil, errors.New("automatic runtime role name exceeds IAM limit")
		}
		a.Name = pulumi.String(prefix + name)
		a.PermissionsBoundary = pulumi.String(boundary)
	}
	return iam.NewRole(ctx, name, &a, opts...)
}

// Function also isolates mutable code. The reset Lambda's existing exact name is
// retained for admin routing and separately verified during automatic enrollment.
func Function(ctx *pulumi.Context, name string, args *lambda.FunctionArgs, opts ...pulumi.ResourceOption) (*lambda.Function, error) {
	prefix, _, e := runtimePins(ctx)
	if e != nil {
		return nil, e
	}
	a := *args
	if prefix != "" && a.Name == nil {
		if len(prefix+name) > 64 {
			return nil, errors.New("automatic Lambda name exceeds AWS limit")
		}
		a.Name = pulumi.String(prefix + name)
	}
	return lambda.NewFunction(ctx, name, &a, opts...)
}
