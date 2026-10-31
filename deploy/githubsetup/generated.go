package githubsetup

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// AttestraProfile is tied to the shipped pulumi/shared and pulumi/backend programs.
// Other consumers must provide reviewed policies rather than infer arbitrary IAM.
const AttestraProfile = "attestra-v1"

type PolicyInput struct {
	Repository, Account, Region, Identity, Stack, Scope, StateBackend, SenderDomain, SenderAddress, Route53ZoneID string
	Domains, KMSKeys                                                                                              []string
	Android                                                                                                       bool
}
type GeneratedPolicy struct {
	ResetFunction                                            string
	Deployment, Boundary, BoundaryARN, RuntimePrefix, Digest string
	DeploymentParts                                          []string
}
type policyStatement struct {
	Sid       string
	Effect    string
	Action    []string
	Resource  []string
	Condition map[string]map[string]any `json:",omitempty"`
}
type policyDocument struct {
	Version   string
	Statement []policyStatement
}

func statement(id string, actions, resources []string) policyStatement {
	return policyStatement{Sid: id, Effect: "Allow", Action: actions, Resource: resources}
}
func conditions(op string, values map[string]any) map[string]map[string]any {
	return map[string]map[string]any{op: values}
}
func document(s []policyStatement) string {
	b, _ := json.Marshal(policyDocument{"2012-10-17", s})
	return string(b)
}
func (p PolicyInput) arn(service, suffix string) string {
	return "arn:aws:" + service + ":" + p.Region + ":" + p.Account + ":" + suffix
}
func (p PolicyInput) functions(prefix string) []string {
	out := []string{p.arn("lambda", "function:"+prefix+"*")}
	if p.Scope == "backend" {
		out = append(out, p.arn("lambda", "function:attestra-backend-reset-"+p.Stack))
	}
	return out
}

// Generate produces canonical, bounded documents; no AWS calls or secrets.
func Generate(p PolicyInput) (GeneratedPolicy, error) {
	var out GeneratedPolicy
	p.Domains = append([]string{}, p.Domains...)
	slices.Sort(p.Domains)
	p.Domains = slices.Compact(p.Domains)
	p.KMSKeys = append([]string{}, p.KMSKeys...)
	slices.Sort(p.KMSKeys)
	p.KMSKeys = slices.Compact(p.KMSKeys)
	safe := regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	if !repoPattern.MatchString(p.Repository) || !regexp.MustCompile(`^[0-9]{12}$`).MatchString(p.Account) || !regexp.MustCompile(`^us-(east|west)-[12]$|^[a-z]{2}-[a-z]+-[1-9]$`).MatchString(p.Region) || !safe.MatchString(p.Stack) || !safe.MatchString(p.Identity) || (p.Scope != "shared" && p.Scope != "backend") {
		return out, errors.New("invalid automatic policy target")
	}
	if p.Stack != p.Identity+"-shared" && p.Scope == "shared" || p.Scope == "backend" && (!strings.HasPrefix(p.Stack, p.Identity+"-") || p.Stack == p.Identity+"-shared") {
		return out, errors.New("policy stack/identity mismatch")
	}
	u, e := url.Parse(p.StateBackend)
	if e != nil || u.Scheme != "s3" || !regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`).MatchString(u.Host) || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") == "" || strings.ContainsAny(u.Path, "*?\\") || strings.Contains(u.Path, "..") {
		return out, errors.New("automatic policies require a concrete S3 state bucket and identity prefix")
	}
	if strings.HasPrefix(u.Host, "id-evidence-") || strings.HasPrefix(u.Host, "app-associations-") {
		return out, errors.New("state bucket overlaps application bucket families; automatic policies cannot isolate it, use a separately reviewed custom policy")
	}
	for _, d := range append(append([]string{}, p.Domains...), p.SenderDomain) {
		if d != "" && !regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*[a-z0-9]$`).MatchString(d) {
			return out, errors.New("invalid policy domain")
		}
	}
	if p.Scope == "backend" && (p.SenderDomain == "" || p.SenderAddress == "" || !strings.HasSuffix(p.SenderAddress, "@"+p.SenderDomain)) {
		return out, errors.New("sender identity required for backend policy")
	}
	if p.Route53ZoneID != "" && !regexp.MustCompile(`^[A-Z0-9]+$`).MatchString(p.Route53ZoneID) {
		return out, errors.New("invalid hosted zone ID")
	}
	for _, key := range p.KMSKeys {
		if !strings.HasPrefix(key, p.arn("kms", "key/")) || !regexp.MustCompile(`^[a-f0-9-]{36}$|^mrk-[a-f0-9]{32}$`).MatchString(strings.TrimPrefix(key, p.arn("kms", "key/"))) {
			return out, errors.New("KMS key must be a concrete ARN in the selected account/region")
		}
	}
	seed := p.Repository + "\x00" + p.Account + "\x00" + p.Region + "\x00" + p.Stack
	out.RuntimePrefix = "af-" + digest([]byte(seed))[:16] + "-"
	functions := p.functions(out.RuntimePrefix)
	if p.Scope == "backend" {
		out.ResetFunction = "attestra-backend-reset-" + p.Stack
	}
	tables := []string{p.arn("dynamodb", "table/admin-runtime-*"), p.arn("dynamodb", "table/environment-registry-*")}
	if p.Scope == "backend" {
		for _, name := range []string{"email-proofs-*", "id-captures-*", "environment-reset-control-*"} {
			tables = append(tables, p.arn("dynamodb", "table/"+name))
		}
	}
	tableResources := append([]string{}, tables...)
	for _, table := range tables {
		tableResources = append(tableResources, table+"/index/*")
	}
	logs := []string{p.arn("logs", "log-group:/aws/lambda/"+out.RuntimePrefix+"*:*")}
	if p.Scope == "backend" {
		logs = append(logs, p.arn("logs", "log-group:/aws/lambda/attestra-backend-reset-"+p.Stack+":*"))
	}
	runtime := []policyStatement{
		{Sid: "NoIdentityAdministration", Effect: "Deny", Action: []string{"iam:*", "sts:AssumeRole*", "sts:GetFederationToken"}, Resource: []string{"*"}},
		statement("RuntimeLogs", []string{"logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"}, logs),
		statement("RuntimeData", []string{"dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:UpdateItem", "dynamodb:DeleteItem", "dynamodb:Query", "dynamodb:Scan", "dynamodb:BatchWriteItem", "dynamodb:ConditionCheckItem"}, tableResources),
	}
	if p.Scope == "shared" {
		runtime = append(runtime, statement("ResetInvocation", []string{"lambda:InvokeFunction"}, []string{p.arn("lambda", "function:attestra-backend-reset-"+p.Identity+"-*")}))
	}
	buckets := []string{}
	queues := []string{p.arn("sqs", "capture-validation*")}
	if p.Scope == "backend" {
		buckets = []string{"arn:aws:s3:::id-evidence-*"}
		if p.Android {
			buckets = append(buckets, "arn:aws:s3:::app-associations-*")
		}
		runtime = append(runtime,
			statement("RuntimeSES", []string{"ses:SendEmail", "ses:SendRawEmail"}, []string{p.arn("ses", "identity/*")}),
			statement("RuntimeCognito", []string{"cognito-idp:AdminCreateUser", "cognito-idp:AdminGetUser", "cognito-idp:AdminUpdateUserAttributes", "cognito-idp:AdminInitiateAuth", "cognito-idp:AdminRespondToAuthChallenge", "cognito-idp:AdminSetUserPassword", "cognito-idp:DescribeUserPool", "cognito-idp:ListUsers", "cognito-idp:AdminDeleteUser"}, []string{p.arn("cognito-idp", "userpool/*")}),
			statement("RuntimeQueue", []string{"sqs:SendMessage", "sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes", "sqs:ChangeMessageVisibility", "sqs:PurgeQueue"}, queues),
			statement("RuntimeBucket", []string{"s3:ListBucket", "s3:ListBucketVersions"}, buckets),
			statement("RuntimeEvidence", []string{"s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:DeleteObjectVersion", "s3:GetObjectVersion"}, []string{"arn:aws:s3:::id-evidence-*/*"}),
			statement("RuntimeReset", []string{"lambda:GetFunctionConcurrency", "lambda:PutFunctionConcurrency", "lambda:DeleteFunctionConcurrency"}, functions),
		)
		inference := statement("Extraction", []string{"bedrock-mantle:CreateInference"}, []string{"*"})
		inference.Condition = conditions("StringEquals", map[string]any{"bedrock-mantle:Model": "google.gemma-4-e2b"})
		runtime = append(runtime, inference)
	}
	for i := range runtime {
		if runtime[i].Sid == "RuntimeSES" {
			runtime[i].Condition = conditions("StringEquals", map[string]any{"ses:FromAddress": p.SenderAddress})
		}
	}
	out.Boundary = document(runtime)
	out.BoundaryARN = "arn:aws:iam::" + p.Account + ":policy/attestra-runtime-" + digest([]byte(out.Boundary))[:24]
	roles := []string{"arn:aws:iam::" + p.Account + ":role/" + out.RuntimePrefix + "*"}
	bucketARN := "arn:aws:s3:::" + u.Host
	objects := bucketARN + "/" + strings.Trim(u.Path, "/") + "/*"
	deploy := []policyStatement{
		statement("StateBucketChecks", []string{"s3:GetBucketLocation", "s3:GetBucketVersioning", "s3:GetEncryptionConfiguration", "s3:GetBucketPublicAccessBlock"}, []string{bucketARN}),
		statement("StateObjects", []string{"s3:GetObject", "s3:PutObject", "s3:DeleteObject"}, []string{objects}),
		statement("RuntimeRoleReadWrite", []string{"iam:ListInstanceProfilesForRole", "iam:GetRole", "iam:DeleteRole", "iam:UpdateRole", "iam:UpdateRoleDescription", "iam:UpdateAssumeRolePolicy", "iam:TagRole", "iam:UntagRole", "iam:ListRoleTags", "iam:ListRolePolicies", "iam:GetRolePolicy", "iam:PutRolePolicy", "iam:DeleteRolePolicy", "iam:ListAttachedRolePolicies"}, roles),
		statement("BoundaryRead", []string{"iam:GetPolicy", "iam:GetPolicyVersion"}, []string{out.BoundaryARN}),
		statement("Functions", []string{"lambda:CreateFunction", "lambda:UpdateFunctionCode", "lambda:UpdateFunctionConfiguration", "lambda:DeleteFunction", "lambda:GetFunction", "lambda:GetFunctionConfiguration", "lambda:GetPolicy", "lambda:AddPermission", "lambda:RemovePermission", "lambda:ListVersionsByFunction", "lambda:ListTags", "lambda:TagResource", "lambda:UntagResource", "lambda:GetFunctionConcurrency", "lambda:PutFunctionConcurrency", "lambda:DeleteFunctionConcurrency", "lambda:GetRuntimeManagementConfig", "lambda:PutRuntimeManagementConfig", "lambda:GetFunctionCodeSigningConfig"}, functions),
		statement("DataInfrastructure", []string{"dynamodb:CreateTable", "dynamodb:UpdateTable", "dynamodb:DeleteTable", "dynamodb:DescribeTable", "dynamodb:DescribeContinuousBackups", "dynamodb:UpdateContinuousBackups", "dynamodb:DescribeTimeToLive", "dynamodb:UpdateTimeToLive", "dynamodb:ListTagsOfResource", "dynamodb:TagResource", "dynamodb:UntagResource", "dynamodb:DescribeContributorInsights", "dynamodb:DescribeKinesisStreamingDestination", "dynamodb:DescribeTableReplicaAutoScaling"}, tables),
		statement("SharedRuntimeBootstrap", []string{"dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:UpdateItem", "dynamodb:ConditionCheckItem"}, []string{p.arn("dynamodb", "table/admin-runtime-*")}),
	}
	list := statement("StateList", []string{"s3:ListBucket"}, []string{bucketARN})
	list.Condition = conditions("StringLikeIfExists", map[string]any{"s3:prefix": []string{strings.Trim(u.Path, "/"), strings.Trim(u.Path, "/") + "/*"}})
	deploy = append(deploy, list)
	create := statement("CreateBoundedRoles", []string{"iam:CreateRole", "iam:PutRolePermissionsBoundary"}, roles)
	create.Condition = conditions("StringEquals", map[string]any{"iam:PermissionsBoundary": out.BoundaryARN})
	deploy = append(deploy, create)
	pass := statement("PassBoundedRoles", []string{"iam:PassRole"}, roles)
	pass.Condition = conditions("StringEquals", map[string]any{"iam:PassedToService": "lambda.amazonaws.com"})
	deploy = append(deploy, pass)
	serviceRole := statement("ServiceLinkedRoles", []string{"iam:CreateServiceLinkedRole"}, []string{"arn:aws:iam::" + p.Account + ":role/aws-service-role/ops.apigateway.amazonaws.com/AWSServiceRoleForAPIGateway", "arn:aws:iam::" + p.Account + ":role/aws-service-role/email.cognito-idp.amazonaws.com/AWSServiceRoleForAmazonCognitoIdpEmailService"})
	serviceRole.Condition = conditions("StringEquals", map[string]any{"iam:AWSServiceName": []string{"ops.apigateway.amazonaws.com", "email.cognito-idp.amazonaws.com"}})
	deploy = append(deploy, serviceRole)
	attach := statement("LambdaLoggingPolicy", []string{"iam:AttachRolePolicy", "iam:DetachRolePolicy"}, roles)
	attach.Condition = conditions("ArnEquals", map[string]any{"iam:PolicyARN": "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"})
	deploy = append(deploy, attach)
	// These APIs use opaque IDs. The review deliberately discloses their wider scope.
	deploy = append(deploy,
		statement("RegionalAPIManagement", []string{"apigateway:GET", "apigateway:POST", "apigateway:PUT", "apigateway:PATCH", "apigateway:DELETE"}, []string{"arn:aws:apigateway:" + p.Region + "::/apis", "arn:aws:apigateway:" + p.Region + "::/apis/*", "arn:aws:apigateway:" + p.Region + "::/domainnames", "arn:aws:apigateway:" + p.Region + "::/domainnames/*", "arn:aws:apigateway:" + p.Region + "::/tags/*"}),
		statement("CognitoManagement", []string{"cognito-idp:DescribeUserPool", "cognito-idp:UpdateUserPool", "cognito-idp:DeleteUserPool", "cognito-idp:CreateUserPoolClient", "cognito-idp:DescribeUserPoolClient", "cognito-idp:UpdateUserPoolClient", "cognito-idp:DeleteUserPoolClient", "cognito-idp:CreateUserPoolDomain", "cognito-idp:UpdateUserPoolDomain", "cognito-idp:DeleteUserPoolDomain", "cognito-idp:GetUserPoolMfaConfig", "cognito-idp:SetUserPoolMfaConfig", "cognito-idp:TagResource", "cognito-idp:UntagResource", "cognito-idp:ListTagsForResource", "cognito-idp:CreateManagedLoginBranding", "cognito-idp:DescribeManagedLoginBranding", "cognito-idp:UpdateManagedLoginBranding", "cognito-idp:DeleteManagedLoginBranding", "cognito-idp:AdminGetUser", "cognito-idp:AdminCreateUser"}, []string{p.arn("cognito-idp", "userpool/*")}),
		statement("Discovery", []string{"cognito-idp:CreateUserPool", "lambda:GetAccountSettings", "cognito-idp:DescribeUserPoolDomain", "cognito-idp:ListUserPools", "lambda:ListFunctions", "lambda:ListEventSourceMappings", "logs:DescribeLogGroups", "acm:ListCertificates"}, []string{"*"}),
		statement("Certificates", []string{"acm:DescribeCertificate", "acm:DeleteCertificate", "acm:ListTagsForCertificate", "acm:AddTagsToCertificate", "acm:RemoveTagsFromCertificate"}, []string{p.arn("acm", "certificate/*")}),
		statement("Publication", []string{"execute-api:Invoke"}, []string{p.arn("execute-api", "*/$default/POST/v1/environments/"+p.Stack+"/changes")}),
	)
	request := statement("RequestSelectedCertificates", []string{"acm:RequestCertificate"}, []string{"*"})
	request.Condition = map[string]map[string]any{"ForAllValues:StringEquals": {"acm:DomainNames": p.Domains}, "Null": {"acm:DomainNames": "false"}, "StringEquals": {"aws:RequestedRegion": p.Region}}
	deploy = append(deploy, request)
	if p.Route53ZoneID != "" {
		deploy = append(deploy, statement("SelectedDNSZone", []string{"route53:GetHostedZone", "route53:ListResourceRecordSets", "route53:ChangeResourceRecordSets"}, []string{"arn:aws:route53:::hostedzone/" + p.Route53ZoneID}), statement("DNSChangeStatus", []string{"route53:GetChange"}, []string{"arn:aws:route53:::change/*"}))
	}
	if len(p.KMSKeys) > 0 {
		s := statement("StateKMS", []string{"kms:Decrypt", "kms:GenerateDataKey"}, p.KMSKeys)
		s.Condition = map[string]map[string]any{"StringEquals": {"kms:ViaService": "s3." + p.Region + ".amazonaws.com"}, "StringLike": {"kms:EncryptionContext:aws:s3:arn": []string{bucketARN, objects}}}
		deploy = append(deploy, s, statement("StateKMSMetadata", []string{"kms:DescribeKey"}, p.KMSKeys))
	}
	if p.Scope == "backend" {
		deploy = append(deploy, backendStatements(p, buckets, queues, functions)...)
	}
	// Global IAM and Route53 exceptions have explicit ARNs. Regional wildcard
	// discovery calls must remain restricted to the selected region.
	for i := range deploy {
		if deploy[i].Sid == "Discovery" {
			deploy[i].Condition = conditions("StringEquals", map[string]any{"aws:RequestedRegion": p.Region})
		}
	}
	out.Deployment = document(deploy)
	// Managed deployment policies are split below the 6144-char limit; the
	// deployment role receives no generated inline policy (10240 aggregate cap).
	var part []policyStatement
	for _, stmt := range deploy {
		candidate := append(append([]policyStatement{}, part...), stmt)
		if len(document(candidate)) > 5500 && len(part) > 0 {
			out.DeploymentParts = append(out.DeploymentParts, document(part))
			part = nil
		}
		part = append(part, stmt)
	}
	if len(part) > 0 {
		out.DeploymentParts = append(out.DeploymentParts, document(part))
	}
	for _, part := range out.DeploymentParts {
		if len(part) > 6144 {
			return GeneratedPolicy{}, errors.New("generated deployment statement exceeds managed policy quota")
		}
	}
	if len(out.DeploymentParts) > 8 || len(out.Boundary) > 6144 {
		return GeneratedPolicy{}, fmt.Errorf("generated IAM documents exceed AWS limits (%d/%d)", len(out.Deployment), len(out.Boundary))
	}
	out.Digest = digest([]byte(AttestraProfile + "\x00" + out.Deployment + "\x00" + out.Boundary))
	return out, nil
}

func backendStatements(p PolicyInput, buckets, queues, functions []string) []policyStatement {
	s := []policyStatement{
		statement("SESIdentityRead", []string{"ses:GetEmailIdentity"}, []string{p.arn("ses", "identity/"+p.SenderDomain)}),
		statement("QueueInfrastructure", []string{"sqs:CreateQueue", "sqs:DeleteQueue", "sqs:GetQueueAttributes", "sqs:SetQueueAttributes", "sqs:GetQueueUrl", "sqs:ListQueueTags", "sqs:TagQueue", "sqs:UntagQueue"}, queues),
		statement("ScheduledWork", []string{"events:PutRule", "events:DescribeRule", "events:DeleteRule", "events:PutTargets", "events:RemoveTargets", "events:ListTargetsByRule", "events:ListTagsForResource", "events:TagResource", "events:UntagResource"}, []string{p.arn("events", "rule/capture-sweep-*"), p.arn("events", "rule/environment-reset-sweep-*")}),
		statement("DLQAlarm", []string{"cloudwatch:PutMetricAlarm", "cloudwatch:DeleteAlarms", "cloudwatch:DescribeAlarms", "cloudwatch:ListTagsForResource", "cloudwatch:TagResource", "cloudwatch:UntagResource"}, []string{p.arn("cloudwatch", "alarm:capture-dlq-alarm-*")}),
		statement("EventMappingReadDelete", []string{"lambda:GetEventSourceMapping", "lambda:TagResource", "lambda:UntagResource", "lambda:ListTags"}, []string{p.arn("lambda", "event-source-mapping:*")}),
		statement("EvidenceInfrastructure", []string{"s3:CreateBucket", "s3:DeleteBucket", "s3:GetBucketLocation", "s3:GetBucketVersioning", "s3:PutBucketVersioning", "s3:GetBucketTagging", "s3:PutBucketTagging", "s3:GetBucketPolicy", "s3:PutBucketPolicy", "s3:DeleteBucketPolicy", "s3:GetBucketPublicAccessBlock", "s3:PutBucketPublicAccessBlock", "s3:GetEncryptionConfiguration", "s3:PutEncryptionConfiguration", "s3:GetBucketOwnershipControls", "s3:PutBucketOwnershipControls", "s3:GetLifecycleConfiguration", "s3:PutLifecycleConfiguration", "s3:GetBucketAcl", "s3:GetBucketCORS", "s3:GetBucketWebsite", "s3:GetAccelerateConfiguration", "s3:GetBucketLogging", "s3:GetBucketRequestPayment", "s3:GetBucketObjectLockConfiguration", "s3:GetReplicationConfiguration"}, buckets),
	}
	mapping := statement("BoundedEventMapping", []string{"lambda:CreateEventSourceMapping", "lambda:UpdateEventSourceMapping", "lambda:DeleteEventSourceMapping"}, []string{"*"})
	mapping.Condition = conditions("ArnLike", map[string]any{"lambda:FunctionArn": functions})
	s = append(s, mapping)
	if p.Android {
		s = append(s, statement("AssociationObject", []string{"s3:GetObject", "s3:PutObject", "s3:DeleteObject", "s3:GetObjectTagging", "s3:PutObjectTagging"}, []string{"arn:aws:s3:::app-associations-*/*"}),
			statement("AssociationCDN", []string{"cloudfront:CreateDistribution", "cloudfront:GetDistribution", "cloudfront:GetDistributionConfig", "cloudfront:UpdateDistribution", "cloudfront:DeleteDistribution", "cloudfront:TagResource", "cloudfront:UntagResource", "cloudfront:ListTagsForResource", "cloudfront:CreateOriginAccessControl", "cloudfront:GetOriginAccessControl", "cloudfront:GetOriginAccessControlConfig", "cloudfront:UpdateOriginAccessControl", "cloudfront:DeleteOriginAccessControl"}, []string{"*"}),
			statement("CDNCertificates", []string{"acm:DescribeCertificate", "acm:DeleteCertificate", "acm:ListTagsForCertificate", "acm:AddTagsToCertificate", "acm:RemoveTagsFromCertificate"}, []string{"arn:aws:acm:us-east-1:" + p.Account + ":certificate/*"}))
		request := statement("RequestCDNCertificate", []string{"acm:RequestCertificate"}, []string{"*"})
		request.Condition = map[string]map[string]any{"ForAllValues:StringEquals": {"acm:DomainNames": p.Domains}, "Null": {"acm:DomainNames": "false"}, "StringEquals": {"aws:RequestedRegion": "us-east-1"}}
		s = append(s, request)
	}
	return s
}
