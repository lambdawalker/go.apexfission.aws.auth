package infrastructure

import (
	"fmt"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/cognito"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/ses"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"net/url"
	"strings"
)

type AuthArgs struct {
	Origin, SenderDomain, SenderAddress, ChallengeArchive string
	ProofKey                                              pulumi.StringInput
}
type Auth struct {
	pulumi.ResourceState
	Pool        *cognito.UserPool
	Client      *cognito.UserPoolClient
	Table       *dynamodb.Table
	Sender      *ses.DomainIdentity
	DKIM        *ses.DomainDkim
	Issuer      pulumi.StringOutput
	Environment pulumi.StringMap
	Policy      pulumi.StringOutput
}

func NewAuth(ctx *pulumi.Context, name string, a AuthArgs, opts ...pulumi.ResourceOption) (*Auth, error) {
	u, e := url.Parse(a.Origin)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.Path != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || a.SenderDomain == "" || !strings.HasSuffix(a.SenderAddress, "@"+a.SenderDomain) || a.ProofKey == nil {
		return nil, fmt.Errorf("auth requires HTTPS origin, matching sender and secret proof key")
	}
	c := &Auth{}
	if e = ctx.RegisterComponentResource("apexfission:auth:Core", name, c, opts...); e != nil {
		return nil, e
	}
	child := []pulumi.ResourceOption{pulumi.Parent(c)}
	c.Table, e = dynamodb.NewTable(ctx, name+"-proofs", &dynamodb.TableArgs{BillingMode: pulumi.String("PAY_PER_REQUEST"), HashKey: pulumi.String("id"), Attributes: dynamodb.TableAttributeArray{&dynamodb.TableAttributeArgs{Name: pulumi.String("id"), Type: pulumi.String("S")}}, Ttl: &dynamodb.TableTtlArgs{AttributeName: pulumi.String("ttl"), Enabled: pulumi.Bool(true)}, ServerSideEncryption: &dynamodb.TableServerSideEncryptionArgs{Enabled: pulumi.Bool(true)}, PointInTimeRecovery: &dynamodb.TablePointInTimeRecoveryArgs{Enabled: pulumi.Bool(true)}}, pulumi.Parent(c), pulumi.Protect(true))
	if e != nil {
		return nil, e
	}
	trigger, e := NewFunction(ctx, name+"-challenge", FunctionArgs{Archive: a.ChallengeArchive, Environment: pulumi.StringMap{"TABLE_NAME": c.Table.Name}, Policy: pulumi.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["dynamodb:GetItem","dynamodb:UpdateItem"],"Resource":%q}]}`, c.Table.Arn)}, child...)
	if e != nil {
		return nil, e
	}
	c.Sender, e = ses.NewDomainIdentity(ctx, name+"-sender", &ses.DomainIdentityArgs{Domain: pulumi.String(a.SenderDomain)}, child...)
	if e != nil {
		return nil, e
	}
	c.DKIM, e = ses.NewDomainDkim(ctx, name+"-dkim", &ses.DomainDkimArgs{Domain: c.Sender.Domain}, child...)
	if e != nil {
		return nil, e
	}
	c.Pool, e = cognito.NewUserPool(ctx, name+"-users", &cognito.UserPoolArgs{UserPoolTier: pulumi.String("ESSENTIALS"), UsernameAttributes: pulumi.StringArray{pulumi.String("email")}, AutoVerifiedAttributes: pulumi.StringArray{pulumi.String("email")}, MfaConfiguration: pulumi.String("OFF"), AdminCreateUserConfig: &cognito.UserPoolAdminCreateUserConfigArgs{AllowAdminCreateUserOnly: pulumi.Bool(true)}, SignInPolicy: &cognito.UserPoolSignInPolicyArgs{AllowedFirstAuthFactors: pulumi.StringArray{pulumi.String("PASSWORD"), pulumi.String("EMAIL_OTP"), pulumi.String("WEB_AUTHN")}}, WebAuthnConfiguration: &cognito.UserPoolWebAuthnConfigurationArgs{RelyingPartyId: pulumi.String(u.Hostname()), UserVerification: pulumi.String("required")}, EmailConfiguration: &cognito.UserPoolEmailConfigurationArgs{EmailSendingAccount: pulumi.String("DEVELOPER"), SourceArn: c.Sender.Arn, FromEmailAddress: pulumi.String(a.SenderAddress)}, LambdaConfig: &cognito.UserPoolLambdaConfigArgs{DefineAuthChallenge: trigger.Arn, CreateAuthChallenge: trigger.Arn, VerifyAuthChallengeResponse: trigger.Arn}, AccountRecoverySetting: &cognito.UserPoolAccountRecoverySettingArgs{RecoveryMechanisms: cognito.UserPoolAccountRecoverySettingRecoveryMechanismArray{&cognito.UserPoolAccountRecoverySettingRecoveryMechanismArgs{Name: pulumi.String("admin_only"), Priority: pulumi.Int(1)}}}}, pulumi.Parent(c), pulumi.Protect(true))
	if e != nil {
		return nil, e
	}
	if _, e = lambda.NewPermission(ctx, name+"-challenge-invoke", &lambda.PermissionArgs{Function: trigger.Name, Action: pulumi.String("lambda:InvokeFunction"), Principal: pulumi.String("cognito-idp.amazonaws.com"), SourceArn: c.Pool.Arn}, child...); e != nil {
		return nil, e
	}
	c.Client, e = cognito.NewUserPoolClient(ctx, name+"-client", &cognito.UserPoolClientArgs{UserPoolId: c.Pool.ID(), GenerateSecret: pulumi.Bool(false), ExplicitAuthFlows: pulumi.StringArray{pulumi.String("ALLOW_CUSTOM_AUTH"), pulumi.String("ALLOW_USER_AUTH"), pulumi.String("ALLOW_REFRESH_TOKEN_AUTH")}, PreventUserExistenceErrors: pulumi.String("ENABLED"), AuthSessionValidity: pulumi.Int(3)}, child...)
	if e != nil {
		return nil, e
	}
	c.Issuer = pulumi.Sprintf("https://%s", c.Pool.Endpoint)
	sesScope := c.Sender.Arn.ApplyT(func(arn string) string {
		parts := strings.SplitN(arn, ":identity/", 2)
		return parts[0] + ":identity/*"
	}).(pulumi.StringOutput)
	c.Environment = pulumi.StringMap{"TABLE_NAME": c.Table.Name, "POOL_ID": c.Pool.ID(), "CLIENT_ID": c.Client.ID(), "ISSUER": c.Issuer, "APP_ORIGIN": pulumi.String(a.Origin), "SENDER_ADDRESS": pulumi.String(a.SenderAddress), "PROOF_KEY": pulumi.ToSecret(a.ProofKey).(pulumi.StringOutput)}
	c.Policy = pulumi.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["dynamodb:GetItem","dynamodb:PutItem","dynamodb:UpdateItem"],"Resource":%q},{"Effect":"Allow","Action":["cognito-idp:AdminGetUser","cognito-idp:AdminCreateUser","cognito-idp:AdminInitiateAuth","cognito-idp:AdminRespondToAuthChallenge"],"Resource":%q},{"Effect":"Allow","Action":"ses:SendEmail","Resource":%q,"Condition":{"StringEquals":{"ses:FromAddress":%q}}}]}`, c.Table.Arn, c.Pool.Arn, sesScope, a.SenderAddress)
	if e = ctx.RegisterResourceOutputs(c, pulumi.Map{"issuer": c.Issuer, "clientId": c.Client.ID(), "proofTable": c.Table.Name, "sesVerification": c.Sender.VerificationToken, "dkimTokens": c.DKIM.DkimTokens}); e != nil {
		return nil, e
	}
	return c, nil
}
