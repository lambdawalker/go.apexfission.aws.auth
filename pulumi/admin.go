package infrastructure

import (
	"fmt"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/cognito"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/dynamodb"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"net/url"
	"strings"
)

type Admin struct {
	LoginDomain pulumi.StringOutput
	Pool        *cognito.UserPool
	Client      *cognito.UserPoolClient
	Table       *dynamodb.Table
	Issuer      pulumi.StringOutput
}

// NewAdmin creates a separate invitation-only MFA identity boundary and config table.
// Operators invite administrators and grant permissions explicitly; no automatic grants.
func NewAdmin(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*Admin, error) {
	return NewAdminWithLogin(ctx, name, "", opts...)
}
func NewAdminWithLogin(ctx *pulumi.Context, name, origin string, opts ...pulumi.ResourceOption) (*Admin, error) {
	if origin != "" {
		u, e := url.Parse(origin)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.Path != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("admin origin must be HTTPS")
		}
	}
	a := &Admin{}
	var e error
	a.Pool, e = cognito.NewUserPool(ctx, name+"-admins", &cognito.UserPoolArgs{UserPoolTier: pulumi.String("ESSENTIALS"), UsernameAttributes: pulumi.StringArray{pulumi.String("email")}, AutoVerifiedAttributes: pulumi.StringArray{pulumi.String("email")}, AdminCreateUserConfig: &cognito.UserPoolAdminCreateUserConfigArgs{AllowAdminCreateUserOnly: pulumi.Bool(true)}, MfaConfiguration: pulumi.String("ON"), SoftwareTokenMfaConfiguration: &cognito.UserPoolSoftwareTokenMfaConfigurationArgs{Enabled: pulumi.Bool(true)}, PasswordPolicy: &cognito.UserPoolPasswordPolicyArgs{MinimumLength: pulumi.Int(14), RequireLowercase: pulumi.Bool(true), RequireUppercase: pulumi.Bool(true), RequireNumbers: pulumi.Bool(true), RequireSymbols: pulumi.Bool(true)}, AccountRecoverySetting: &cognito.UserPoolAccountRecoverySettingArgs{RecoveryMechanisms: cognito.UserPoolAccountRecoverySettingRecoveryMechanismArray{&cognito.UserPoolAccountRecoverySettingRecoveryMechanismArgs{Name: pulumi.String("admin_only"), Priority: pulumi.Int(1)}}}}, append([]pulumi.ResourceOption{pulumi.Protect(true)}, opts...)...)
	if e != nil {
		return nil, e
	}
	clientArgs := &cognito.UserPoolClientArgs{UserPoolId: a.Pool.ID(), GenerateSecret: pulumi.Bool(false), ExplicitAuthFlows: pulumi.StringArray{pulumi.String("ALLOW_USER_SRP_AUTH"), pulumi.String("ALLOW_REFRESH_TOKEN_AUTH")}, PreventUserExistenceErrors: pulumi.String("ENABLED")}
	if origin != "" {
		clientArgs.AllowedOauthFlowsUserPoolClient = pulumi.Bool(true)
		clientArgs.AllowedOauthFlows = pulumi.StringArray{pulumi.String("code")}
		clientArgs.AllowedOauthScopes = pulumi.StringArray{pulumi.String("openid"), pulumi.String("email"), pulumi.String("aws.cognito.signin.user.admin")}
		clientArgs.CallbackUrls = pulumi.StringArray{pulumi.String(origin + "/admin")}
		clientArgs.LogoutUrls = pulumi.StringArray{pulumi.String(origin + "/admin")}
		clientArgs.SupportedIdentityProviders = pulumi.StringArray{pulumi.String("COGNITO")}
	}
	a.Client, e = cognito.NewUserPoolClient(ctx, name+"-admin-client", clientArgs, opts...)
	if e != nil {
		return nil, e
	}
	a.Table, e = dynamodb.NewTable(ctx, name+"-config", &dynamodb.TableArgs{BillingMode: pulumi.String("PAY_PER_REQUEST"), HashKey: pulumi.String("pk"), RangeKey: pulumi.String("sk"), Attributes: dynamodb.TableAttributeArray{&dynamodb.TableAttributeArgs{Name: pulumi.String("pk"), Type: pulumi.String("S")}, &dynamodb.TableAttributeArgs{Name: pulumi.String("sk"), Type: pulumi.String("S")}}, ServerSideEncryption: &dynamodb.TableServerSideEncryptionArgs{Enabled: pulumi.Bool(true)}, PointInTimeRecovery: &dynamodb.TablePointInTimeRecoveryArgs{Enabled: pulumi.Bool(true)}}, append([]pulumi.ResourceOption{pulumi.Protect(true)}, opts...)...)
	if e != nil {
		return nil, e
	}
	a.Issuer = pulumi.Sprintf("https://%s", a.Pool.Endpoint)
	if origin != "" {
		prefix := a.Pool.ID().ToStringOutput().ApplyT(func(id string) string { return "apex-" + strings.ToLower(strings.ReplaceAll(id, "_", "-")) }).(pulumi.StringOutput)
		domain, e := cognito.NewUserPoolDomain(ctx, name+"-login", &cognito.UserPoolDomainArgs{UserPoolId: a.Pool.ID(), Domain: prefix}, opts...)
		if e != nil {
			return nil, e
		}
		a.LoginDomain = pulumi.All(domain.Domain, a.Pool.Endpoint).ApplyT(func(values []any) string {
			endpoint := values[1].(string)
			region := strings.Split(endpoint, ".")[1]
			return "https://" + values[0].(string) + ".auth." + region + ".amazoncognito.com"
		}).(pulumi.StringOutput)
	}
	return a, nil
}
