package guard

import (
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/acm"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Certificate references an externally issued certificate when supplied. External
// DNS operators must validate it before deployment; its lifetime stays external.
func Certificate(ctx *pulumi.Context, key, name string, args *acm.CertificateArgs, opts ...pulumi.ResourceOption) (*acm.Certificate, error) {
	if arn := config.New(ctx, "").Get(key); arn != "" {
		return acm.GetCertificate(ctx, name, pulumi.ID(arn), nil, opts...)
	}
	return acm.NewCertificate(ctx, name, args, opts...)
}
