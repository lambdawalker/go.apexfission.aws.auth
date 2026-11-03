package infrastructure

import "github.com/lambdawalker/go.infra.composer/adapters/pulumi/awslabels"

import (
	"errors"
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/assetlinks"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/apigatewayv2"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/cognito"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/route53"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/ses"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func Deploy(ctx *pulumi.Context, cfg Config, opts Options) (*Deployment, error) {
	if opts.Environment == "" {
		opts.Environment = ctx.Stack()
	}
	if err := Validate(cfg, opts); err != nil {
		return nil, err
	}
	ctx, labels, err := awslabels.Ensure(ctx, opts.Environment, cfg.Get("appOrigin"), "authentication", "go.apexfission.aws.auth")
	if err != nil {
		return nil, err
	}
	d, err := deploy(ctx, cfg, opts)
	if err == nil && labels != nil {
		err = labels.Export(ctx, "resourceInventory")
	}
	return d, err
}
func deploy(ctx *pulumi.Context, cfg Config, opts Options) (*Deployment, error) {
	origin := cfg.Require("appOrigin")
	senderDomain := cfg.Require("senderDomain")
	senderAddress := cfg.Require("senderAddress")
	zone := cfg.Get("route53ZoneId")
	if err := validate(origin, senderDomain, senderAddress); err != nil {
		return nil, err
	}
	challengeArchive, _ := filepath.Abs(filepath.Join(opts.ArtifactDir, "challenge.zip"))
	authArchives := map[string]string{}
	for _, name := range []string{"auth-email-start", "auth-email-complete", "auth-passkey-start", "auth-passkey-complete", "auth-refresh", "auth-status", "auth-password-start", "auth-password-complete", "auth-password-set"} {
		authArchives[name], _ = filepath.Abs(filepath.Join(opts.ArtifactDir, name+".zip"))
	}
	archives := map[string]string{}
	for _, name := range []string{"passkeyoptions", "passkeycomplete"} {
		archives[name], _ = filepath.Abs(filepath.Join(opts.ArtifactDir, name+".zip"))
	}
	for _, path := range []string{archives["passkeyoptions"], archives["passkeycomplete"], challengeArchive, authArchives["auth-email-start"], authArchives["auth-email-complete"], authArchives["auth-passkey-start"], authArchives["auth-passkey-complete"], authArchives["auth-refresh"], authArchives["auth-status"], authArchives["auth-password-start"], authArchives["auth-password-complete"], authArchives["auth-password-set"]} {
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("build Lambda archives with scripts/build.sh or scripts/build.bat first: %s: %w", path, err)
		}
	}
	certificate, err := apiCertificate(ctx, cfg.Get("apiDomain"), zone)
	if err != nil {
		return nil, err
	}
	// Absence preserves existing sites; an explicit empty array publishes a revocation file.
	if cfg.Get("androidAssociations") != "" {
		var apps []assetlinks.Application
		if err := cfg.GetObject("androidAssociations", &apps); err != nil {
			return nil, err
		}
		if err := deployAssetLinks(ctx, origin, zone, apps); err != nil {
			return nil, err
		}
	}
	region := aws.GetRegionOutput(ctx, aws.GetRegionOutputArgs{}).Name()
	account := aws.GetCallerIdentityOutput(ctx, aws.GetCallerIdentityOutputArgs{}).AccountId()
	table, err := dynamodb.NewTable(ctx, "email-proofs", &dynamodb.TableArgs{BillingMode: pulumi.String("PAY_PER_REQUEST"), HashKey: pulumi.String("id"), Attributes: dynamodb.TableAttributeArray{&dynamodb.TableAttributeArgs{Name: pulumi.String("id"), Type: pulumi.String("S")}}, Ttl: &dynamodb.TableTtlArgs{AttributeName: pulumi.String("ttl"), Enabled: pulumi.Bool(true)}, PointInTimeRecovery: &dynamodb.TablePointInTimeRecoveryArgs{Enabled: pulumi.Bool(true)}, ServerSideEncryption: &dynamodb.TableServerSideEncryptionArgs{Enabled: pulumi.Bool(true)}}, awslabels.Option(ctx))
	if err != nil {
		return nil, err
	}
	control, err := resetControl(ctx)
	if err != nil {
		return nil, err
	}
	reset := &ResetInventory{Control: control, Tables: pulumi.StringArray{table.Name}, TableARNs: pulumi.StringArray{table.Arn}}
	sender, err := ses.NewDomainIdentity(ctx, "email-sender", &ses.DomainIdentityArgs{Domain: pulumi.String(senderDomain)}, awslabels.Option(ctx))
	if err != nil {
		return nil, err
	}
	dkim, err := ses.NewDomainDkim(ctx, "email-sender-dkim", &ses.DomainDkimArgs{Domain: sender.Domain}, awslabels.Option(ctx))
	if err != nil {
		return nil, err
	}
	if zone != "" {
		_, err = route53.NewRecord(ctx, "ses-verify", &route53.RecordArgs{ZoneId: pulumi.String(zone), Name: pulumi.String("_amazonses." + senderDomain), Type: pulumi.String("TXT"), Ttl: pulumi.Int(300), Records: pulumi.StringArray{sender.VerificationToken}}, awslabels.Option(ctx))
		if err != nil {
			return nil, err
		}
		for n := 0; n < 3; n++ {
			idx := n
			token := dkim.DkimTokens.ApplyT(func(v []string) string { return v[idx] }).(pulumi.StringOutput)
			_, err = route53.NewRecord(ctx, fmt.Sprintf("ses-dkim-%d", n), &route53.RecordArgs{ZoneId: pulumi.String(zone), Name: pulumi.Sprintf("%s._domainkey.%s", token, senderDomain), Type: pulumi.String("CNAME"), Ttl: pulumi.Int(300), Records: pulumi.StringArray{pulumi.Sprintf("%s.dkim.amazonses.com", token)}}, awslabels.Option(ctx))
			if err != nil {
				return nil, err
			}
		}
	}
	trust := pulumi.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}`)
	challengeRole, err := iam.NewRole(ctx, "challenge-role", &iam.RoleArgs{AssumeRolePolicy: trust}, awslabels.Option(ctx))
	if err != nil {
		return nil, err
	}
	_, err = iam.NewRolePolicyAttachment(ctx, "challenge-logs", &iam.RolePolicyAttachmentArgs{Role: challengeRole.Name, PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole")}, awslabels.Option(ctx))
	if err != nil {
		return nil, err
	}
	trigger, err := lambda.NewFunction(ctx, "cognito-grant-challenge", &lambda.FunctionArgs{Runtime: pulumi.String("provided.al2023"), Handler: pulumi.String("bootstrap"), Architectures: pulumi.StringArray{pulumi.String("arm64")}, Role: challengeRole.Arn, Code: pulumi.NewFileArchive(challengeArchive), Timeout: pulumi.Int(10), Environment: &lambda.FunctionEnvironmentArgs{Variables: pulumi.StringMap{"TABLE_NAME": table.Name}}}, awslabels.Option(ctx))
	if err != nil {
		return nil, err
	}
	reset.addFunction(trigger)
	permission, err := lambda.NewPermission(ctx, "allow-cognito-challenge", &lambda.PermissionArgs{Action: pulumi.String("lambda:InvokeFunction"), Function: trigger.Name, Principal: pulumi.String("cognito-idp.amazonaws.com"), SourceAccount: account, SourceArn: pulumi.Sprintf("arn:aws:cognito-idp:%s:%s:userpool/*", region, account)}, awslabels.Option(ctx))
	if err != nil {
		return nil, err
	}
	challengePolicy := pulumi.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"dynamodb:UpdateItem","Resource":%q}]}`, table.Arn)
	challengeGrant, err := iam.NewRolePolicy(ctx, "challenge-permissions", &iam.RolePolicyArgs{Role: challengeRole.ID(), Policy: challengePolicy}, awslabels.Option(ctx))
	if err != nil {
		return nil, err
	}
	pool, err := cognito.NewUserPool(ctx, "attesta-users", &cognito.UserPoolArgs{
		PasswordPolicy: &cognito.UserPoolPasswordPolicyArgs{MinimumLength: pulumi.Int(8), RequireLowercase: pulumi.Bool(false), RequireUppercase: pulumi.Bool(false), RequireNumbers: pulumi.Bool(false), RequireSymbols: pulumi.Bool(false)},
		UserPoolTier:   pulumi.String("ESSENTIALS"), UsernameAttributes: pulumi.StringArray{pulumi.String("email")}, AutoVerifiedAttributes: pulumi.StringArray{pulumi.String("email")}, MfaConfiguration: pulumi.String("OFF"),
		// Cognito requires PASSWORD in this policy even when users are created
		// without passwords. EMAIL_OTP permits passwordless account recovery.
		SignInPolicy:          &cognito.UserPoolSignInPolicyArgs{AllowedFirstAuthFactors: pulumi.StringArray{pulumi.String("PASSWORD"), pulumi.String("EMAIL_OTP"), pulumi.String("WEB_AUTHN")}},
		AdminCreateUserConfig: &cognito.UserPoolAdminCreateUserConfigArgs{AllowAdminCreateUserOnly: pulumi.Bool(true)},
		EmailConfiguration:    &cognito.UserPoolEmailConfigurationArgs{EmailSendingAccount: pulumi.String("DEVELOPER"), SourceArn: sender.Arn, FromEmailAddress: pulumi.String(senderAddress)},
		LambdaConfig:          &cognito.UserPoolLambdaConfigArgs{DefineAuthChallenge: trigger.Arn, CreateAuthChallenge: trigger.Arn, VerifyAuthChallengeResponse: trigger.Arn},
		WebAuthnConfiguration: &cognito.UserPoolWebAuthnConfigurationArgs{RelyingPartyId: pulumi.String(strings.TrimPrefix(origin, "https://")), UserVerification: pulumi.String("required")},

		// Disable self-service password resets; accounts are created without
		// passwords and the app uses email OTP to recover sign-in.
		AccountRecoverySetting: &cognito.UserPoolAccountRecoverySettingArgs{RecoveryMechanisms: cognito.UserPoolAccountRecoverySettingRecoveryMechanismArray{
			&cognito.UserPoolAccountRecoverySettingRecoveryMechanismArgs{Name: pulumi.String("admin_only"), Priority: pulumi.Int(1)},
		}},
	}, pulumi.DependsOn([]pulumi.Resource{permission, challengeGrant}), awslabels.Option(ctx))
	if err != nil {
		return nil, err
	}
	reset.Pool = pool.ID()
	reset.PoolARN = pool.Arn
	client, err := cognito.NewUserPoolClient(ctx, "public-auth-client", &cognito.UserPoolClientArgs{UserPoolId: pool.ID(), GenerateSecret: pulumi.Bool(false), ExplicitAuthFlows: pulumi.StringArray{pulumi.String("ALLOW_CUSTOM_AUTH"), pulumi.String("ALLOW_USER_AUTH"), pulumi.String("ALLOW_REFRESH_TOKEN_AUTH")}, PreventUserExistenceErrors: pulumi.String("ENABLED"), AuthSessionValidity: pulumi.Int(3)}, awslabels.Option(ctx))
	if err != nil {
		return nil, err
	}
	httpAPI, err := apigatewayv2.NewApi(ctx, "email-http-api", &apigatewayv2.ApiArgs{ProtocolType: pulumi.String("HTTP"), CorsConfiguration: &apigatewayv2.ApiCorsConfigurationArgs{AllowOrigins: pulumi.StringArray{pulumi.String(origin)}, AllowMethods: pulumi.StringArray{pulumi.String("POST"), pulumi.String("GET")}, AllowHeaders: pulumi.StringArray{pulumi.String("content-type"), pulumi.String("authorization")}, MaxAge: pulumi.Int(300)}}, awslabels.Option(ctx))
	if err != nil {
		return nil, err
	}
	// Registration is authorized by Cognito with the caller's access token, not IAM.
	// Keep each endpoint in a separate Lambda, as with email verification.
	for _, spec := range []struct{ name, path string }{{"passkeyoptions", "passkeys/options"}, {"passkeycomplete", "passkeys/complete"}} {
		role, e := iam.NewRole(ctx, spec.name+"-role", &iam.RoleArgs{AssumeRolePolicy: trust}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
		_, e = iam.NewRolePolicyAttachment(ctx, spec.name+"-logs", &iam.RolePolicyAttachmentArgs{Role: role.Name, PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole")}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
		fn, e := lambda.NewFunction(ctx, spec.name, &lambda.FunctionArgs{Runtime: pulumi.String("provided.al2023"), Handler: pulumi.String("bootstrap"), Architectures: pulumi.StringArray{pulumi.String("arm64")}, Role: role.Arn, Code: pulumi.NewFileArchive(archives[spec.name]), Timeout: pulumi.Int(20)}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
		reset.addFunction(fn)
		integration, e := apigatewayv2.NewIntegration(ctx, spec.name+"-integration", &apigatewayv2.IntegrationArgs{ApiId: httpAPI.ID(), IntegrationType: pulumi.String("AWS_PROXY"), IntegrationUri: fn.InvokeArn, PayloadFormatVersion: pulumi.String("2.0")}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
		_, e = apigatewayv2.NewRoute(ctx, spec.name+"-route", &apigatewayv2.RouteArgs{ApiId: httpAPI.ID(), RouteKey: pulumi.String("POST /" + spec.path), Target: pulumi.Sprintf("integrations/%s", integration.ID())}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
		_, e = lambda.NewPermission(ctx, "allow-"+spec.name, &lambda.PermissionArgs{Action: pulumi.String("lambda:InvokeFunction"), Function: fn.Name, Principal: pulumi.String("apigateway.amazonaws.com"), SourceArn: pulumi.Sprintf("%s/*/POST/%s", httpAPI.ExecutionArn, spec.path)}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
	}
	for _, spec := range []struct{ name, path string }{{"auth-email-start", "email/start"}, {"auth-email-complete", "email/complete"}, {"auth-passkey-start", "passkey/start"}, {"auth-passkey-complete", "passkey/complete"}, {"auth-refresh", "refresh"}, {"auth-status", "status"}, {"auth-password-start", "password/start"}, {"auth-password-complete", "password/complete"}, {"auth-password-set", "password/set"}} {
		role, e := iam.NewRole(ctx, spec.name+"-role", &iam.RoleArgs{AssumeRolePolicy: trust}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
		_, e = iam.NewRolePolicyAttachment(ctx, spec.name+"-logs", &iam.RolePolicyAttachmentArgs{Role: role.Name, PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole")}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
		variables := pulumi.StringMap{"CLIENT_ID": client.ID(), "AUTH_ROUTE": pulumi.String(spec.path)}
		dependencies := []pulumi.Resource{}
		if spec.path == "status" || spec.path == "password/set" {
			variables["POOL_ID"] = pool.ID()
			variables["ISSUER"] = pulumi.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, pool.ID())
			variables["ENVIRONMENT"] = pulumi.String(opts.Environment)
			variables["RUNTIME_TABLE"] = opts.RuntimeTableName
			variables["TABLE_NAME"] = table.Name
			if spec.path == "status" && opts.CaptureTableName != nil {
				variables["CAPTURE_TABLE"] = opts.CaptureTableName
			}
			captureARN := pulumi.String("").ToStringOutput()
			if opts.CaptureTableARN != nil {
				captureARN = opts.CaptureTableARN.ToStringOutput()
			}
			policy := pulumi.All(table.Arn, captureARN, pool.Arn, opts.RuntimeTableARN).ApplyT(func(v []interface{}) string {
				return enrollmentPolicy(spec.path, opts.Environment, v[3].(string), v[0].(string), v[1].(string), v[2].(string))
			}).(pulumi.StringOutput)
			grant, e := iam.NewRolePolicy(ctx, spec.name+"-enrollment", &iam.RolePolicyArgs{Role: role.Name, Policy: policy}, awslabels.Option(ctx))
			if e != nil {
				return nil, e
			}
			dependencies = append(dependencies, grant)
		}
		fn, e := lambda.NewFunction(ctx, spec.name, &lambda.FunctionArgs{Runtime: pulumi.String("provided.al2023"), Handler: pulumi.String("bootstrap"), Architectures: pulumi.StringArray{pulumi.String("arm64")}, Role: role.Arn, Code: pulumi.NewFileArchive(authArchives[spec.name]), Timeout: pulumi.Int(20), Environment: &lambda.FunctionEnvironmentArgs{Variables: variables}}, pulumi.DependsOn(dependencies), awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
		reset.addFunction(fn)
		integration, e := apigatewayv2.NewIntegration(ctx, spec.name+"-integration", &apigatewayv2.IntegrationArgs{ApiId: httpAPI.ID(), IntegrationType: pulumi.String("AWS_PROXY"), IntegrationUri: fn.InvokeArn, PayloadFormatVersion: pulumi.String("2.0")}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
		_, e = apigatewayv2.NewRoute(ctx, spec.name+"-route", &apigatewayv2.RouteArgs{ApiId: httpAPI.ID(), RouteKey: pulumi.String("POST /auth/" + spec.path), Target: pulumi.Sprintf("integrations/%s", integration.ID())}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
		_, e = lambda.NewPermission(ctx, "allow-"+spec.name, &lambda.PermissionArgs{Action: pulumi.String("lambda:InvokeFunction"), Function: fn.Name, Principal: pulumi.String("apigateway.amazonaws.com"), SourceArn: pulumi.Sprintf("%s/*/POST/auth/%s", httpAPI.ExecutionArn, spec.path)}, awslabels.Option(ctx))
		if e != nil {
			return nil, e
		}
	}
	if err = deployRuntime(ctx, cfg, opts, httpAPI); err != nil {
		return nil, err
	}
	var resetFn *lambda.Function
	resetName := pulumi.String(opts.ResetFunctionName).ToStringOutput()
	if !opts.DeferReset {
		resetFn, err = deployReset(ctx, cfg, opts, reset)
		if err != nil {
			return nil, err
		}
		resetName = resetFn.Name
	}
	stage, err := apigatewayv2.NewStage(ctx, "email-stage", &apigatewayv2.StageArgs{ApiId: httpAPI.ID(), Name: pulumi.String("$default"), AutoDeploy: pulumi.Bool(true), DefaultRouteSettings: &apigatewayv2.StageDefaultRouteSettingsArgs{ThrottlingBurstLimit: pulumi.Int(20), ThrottlingRateLimit: pulumi.Float64(10)}}, awslabels.Option(ctx))
	if err != nil {
		return nil, err
	}
	if indexURL := cfg.Get("indexUrl"); indexURL != "" {
		ctx.Export("indexUrl", pulumi.String(indexURL))
	}
	defaults := deploymentRuntimeDefaults(opts.Environment, captureSetting(cfg, opts))
	// New defaults use generic localized disclosure; admin overrides remain revisioned.
	defaults.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if e := defaults.Validate(); e != nil {
		return nil, e
	}
	ctx.Export("runtimeDefaults", pulumi.Any(defaults))
	ctx.Export("rawApiUrl", httpAPI.ApiEndpoint)
	if err := apiDomainMapping(ctx, cfg.Get("apiDomain"), zone, certificate, httpAPI, stage); err != nil {
		return nil, err
	}
	ctx.Export("userPoolId", pool.ID())
	ctx.Export("clientId", client.ID())
	ctx.Export("sesVerificationRecord", pulumi.Sprintf("_amazonses.%s TXT %s", senderDomain, sender.VerificationToken))
	ctx.Export("sesDkimTokens", dkim.DkimTokens)
	apiURL := httpAPI.ApiEndpoint
	if domain := cfg.Get("apiDomain"); domain != "" {
		apiURL = pulumi.String("https://" + domain).ToStringOutput()
	}
	return &Deployment{CaptureIntegration: map[string]pulumi.StringOutput{"api_id": httpAPI.ID().ToStringOutput(), "api_execution_arn": httpAPI.ExecutionArn, "pool_id": pool.ID().ToStringOutput(), "client_id": client.ID().ToStringOutput(), "issuer": pulumi.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, pool.ID()), "region": region, "reset_table_name": control.Name, "reset_table_arn": control.Arn}, RuntimeTableName: opts.RuntimeTableName.ToStringOutput(), RuntimeTableARN: opts.RuntimeTableARN.ToStringOutput(), APIURL: apiURL, PoolID: pool.ID().ToStringOutput(), ClientID: client.ID().ToStringOutput(), Issuer: pulumi.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, pool.ID()), ResetFunctionName: resetName, ResetInventory: reset, Integration: map[string]pulumi.StringOutput{"pool_id": pool.ID().ToStringOutput(), "pool_arn": pool.Arn, "client_id": client.ID().ToStringOutput(), "grants_table_name": table.Name, "grants_table_arn": table.Arn, "api_id": httpAPI.ID().ToStringOutput(), "api_execution_arn": httpAPI.ExecutionArn, "sender_identity_arn": sender.Arn, "region": region, "account_id": account}, API: httpAPI, Stage: stage, ResetFunction: resetFn}, nil
}
func validate(origin, domain, address string) error {
	u, e := url.Parse(origin)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return errors.New("appOrigin must be an HTTPS origin")
	}
	if domain == "" || strings.ToLower(address) != address || !strings.HasSuffix(address, "@"+domain) {
		return errors.New("senderAddress must belong to senderDomain")
	}
	return nil
}
