package wizard

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (x *execution) verifyState() error {
	var caller struct{ Account string }
	if e := x.aws(&caller, "sts", "get-caller-identity"); e != nil {
		return e
	}
	if caller.Account != x.c.Account {
		return errors.New("AWS STS account mismatch")
	}
	u, _ := url.Parse(x.c.StateBackend)
	bucket := u.Host
	if e := x.aws(nil, "s3api", "head-bucket", "--bucket", bucket, "--expected-bucket-owner", x.c.Account); e != nil {
		return fmt.Errorf("state bucket ownership/read access: %w", e)
	}
	var location struct{ LocationConstraint string }
	if e := x.aws(&location, "s3api", "get-bucket-location", "--bucket", bucket, "--expected-bucket-owner", x.c.Account); e != nil {
		return e
	}
	region := location.LocationConstraint
	if region == "" {
		region = "us-east-1"
	}
	if region == "EU" {
		region = "eu-west-1"
	}
	if region != x.c.Region {
		return errors.New("state bucket region does not match selected region")
	}
	var encryption struct {
		ServerSideEncryptionConfiguration struct {
			Rules []struct {
				ApplyServerSideEncryptionByDefault struct{ SSEAlgorithm, KMSMasterKeyID string }
			}
		}
	}
	if e := x.aws(&encryption, "s3api", "get-bucket-encryption", "--bucket", bucket, "--expected-bucket-owner", x.c.Account); e != nil {
		return e
	}
	encrypted := false
	for _, rule := range encryption.ServerSideEncryptionConfiguration.Rules {
		algorithm := rule.ApplyServerSideEncryptionByDefault.SSEAlgorithm
		if algorithm == "AES256" || algorithm == "aws:kms" || algorithm == "aws:kms:dsse" {
			encrypted = true
		}
		if (algorithm == "aws:kms" || algorithm == "aws:kms:dsse") && x.o.GitHubPolicyProfile == "attestra-v1" {
			key := rule.ApplyServerSideEncryptionByDefault.KMSMasterKeyID
			if key == "" {
				key = "alias/aws/s3"
			}
			var result struct{ KeyMetadata struct{ Arn string } }
			if e := x.aws(&result, "kms", "describe-key", "--key-id", key); e != nil {
				return fmt.Errorf("resolve state encryption key for automatic deployment policy: %w", e)
			}
			if result.KeyMetadata.Arn == "" {
				return errors.New("state KMS key ARN unavailable")
			}
			x.stateKMSKeys = append(x.stateKMSKeys, result.KeyMetadata.Arn)
		}
	}
	if !encrypted {
		return errors.New("state bucket must have default server-side encryption")
	}
	var version struct{ Status string }
	if e := x.aws(&version, "s3api", "get-bucket-versioning", "--bucket", bucket, "--expected-bucket-owner", x.c.Account); e != nil {
		return e
	}
	if version.Status != "Enabled" {
		return errors.New("existing state bucket must have versioning enabled; wizard never creates buckets")
	}
	var public struct {
		PublicAccessBlockConfiguration struct{ BlockPublicAcls, IgnorePublicAcls, BlockPublicPolicy, RestrictPublicBuckets bool }
	}
	if e := x.aws(&public, "s3api", "get-public-access-block", "--bucket", bucket, "--expected-bucket-owner", x.c.Account); e != nil {
		return e
	}
	p := public.PublicAccessBlockConfiguration
	if !p.BlockPublicAcls || !p.IgnorePublicAcls || !p.BlockPublicPolicy || !p.RestrictPublicBuckets {
		return errors.New("state bucket must block all public access")
	}
	return nil
}

// A conditional S3 identity lock serializes configuration and registry work across
// all environments/machines. Stale locks fail closed and require manual review.
func (x *execution) acquireLock() (func(), error) {
	u, _ := url.Parse(x.c.backend())
	key := strings.TrimPrefix(u.Path, "/") + "/wizard.lock"
	token := make([]byte, 24)
	if _, e := rand.Read(token); e != nil {
		return nil, e
	}
	body, _ := json.Marshal(map[string]string{"token": hex.EncodeToString(token), "identity": x.c.identity(), "created": time.Now().UTC().Format(time.RFC3339)})
	f, e := os.CreateTemp("", "attestra-lock-*")
	if e != nil {
		return nil, e
	}
	defer os.Remove(f.Name())
	if _, e = f.Write(body); e != nil {
		f.Close()
		return nil, e
	}
	f.Close()
	var put struct{ ETag string }
	if e = x.aws(&put, "s3api", "put-object", "--bucket", u.Host, "--key", key, "--expected-bucket-owner", x.c.Account, "--if-none-match", "*", "--body", f.Name()); e != nil {
		return nil, fmt.Errorf("identity lock unavailable; inspect s3://%s/%s and pending registry receipts before manual stale-lock recovery: %w", u.Host, key, e)
	}
	if put.ETag == "" {
		return nil, errors.New("S3 lock returned no ETag; inspect manually")
	}
	return func() {
		if e := x.aws(nil, "s3api", "delete-object", "--bucket", u.Host, "--key", key, "--expected-bucket-owner", x.c.Account, "--if-match", put.ETag); e != nil {
			fmt.Fprintln(x.o.Output, "Lock release failed; inspect identity lock before retrying.")
		}
	}, nil
}
func (x *execution) receiptPath(suffix string) string {
	return filepath.Join(x.o.Root, ".attestra", x.c.stack(false)+"-"+suffix+".json")
}
func (x *execution) sender() error {
	if x.c.SenderDomain == "" || x.c.SenderAddress == "" {
		return errors.New("backend deployment requires externally verified --sender-domain and --sender-address; wizard never owns SES resources")
	}
	if !strings.HasSuffix(strings.ToLower(x.c.SenderAddress), "@"+x.c.SenderDomain) {
		return errors.New("sender-address must belong to sender-domain")
	}
	var result struct{ VerifiedForSendingStatus bool }
	if e := x.aws(&result, "sesv2", "get-email-identity", "--email-identity", x.c.SenderDomain); e != nil {
		return e
	}
	if !result.VerifiedForSendingStatus {
		return errors.New("external SES sender domain is not verified for sending in selected account/region")
	}
	return nil
}
func (x *execution) exportAndroid(out, shared map[string]any) error {
	_, _, _, origin := x.c.domains()
	values := map[string]string{"API_BASE_URL": str(out, "apiUrl"), "ENV_INDEX_URL": str(shared, "indexUrl"), "COGNITO_USER_POOL_ID": str(out, "userPoolId"), "COGNITO_CLIENT_ID": str(out, "clientId"), "AWS_REGION": x.c.Region, "APP_LINK_HOST": strings.TrimPrefix(origin, "https://"), "ENVIRONMENT": x.c.stack(false)}
	defaults := map[string]any{}
	if raw, ok := out["runtimeDefaults"].(map[string]any); ok {
		defaults = raw
	} else {
		b, _ := json.Marshal(out["runtimeDefaults"])
		_ = json.Unmarshal(b, &defaults)
	}
	captureKind := "id_card"
	if capture, ok := defaults["capture"].(map[string]any); ok {
		if kind, ok := capture["documentType"].(string); ok && kind != "" {
			captureKind = kind
		}
	}
	enabled := false
	if features, ok := defaults["features"].(map[string]any); ok {
		enabled, _ = features["idCapture"].(bool)
	}
	values["attestraEnvironment"] = values["ENVIRONMENT"]
	values["attestraApiBaseUrl"] = values["API_BASE_URL"]
	values["attestraLinkHost"] = values["APP_LINK_HOST"]
	values["attestraAwsRegion"] = values["AWS_REGION"]
	values["attestraCognitoUserPoolId"] = values["COGNITO_USER_POOL_ID"]
	values["attestraCognitoClientId"] = values["COGNITO_CLIENT_ID"]
	values["attestraCaptureEnabled"] = strconv.FormatBool(enabled)
	values["attestraCaptureDocumentType"] = captureKind
	values["attestraIndexUrl"] = values["ENV_INDEX_URL"]
	// Original Android keys are preserved; additional aliases are also public.
	values["ATTESTRA_API_BASE_URL"] = values["API_BASE_URL"]
	values["ATTESTRA_ENV_INDEX_URL"] = values["ENV_INDEX_URL"]
	values["ATTESTRA_COGNITO_USER_POOL_ID"] = values["COGNITO_USER_POOL_ID"]
	values["ATTESTRA_COGNITO_CLIENT_ID"] = values["COGNITO_CLIENT_ID"]
	values["ATTESTRA_AWS_REGION"] = values["AWS_REGION"]
	values["ATTESTRA_APP_LINK_HOST"] = values["APP_LINK_HOST"]
	values["ATTESTRA_ENVIRONMENT"] = values["ENVIRONMENT"]
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b := strings.Builder{}
	for _, k := range keys {
		if strings.ContainsAny(values[k], "\r\n") {
			return errors.New("invalid Android property output")
		}
		fmt.Fprintf(&b, "%s=%s\n", k, values[k])
	}
	return os.WriteFile(filepath.Join(x.o.Root, ".attestra", x.c.stack(false)+"-android.properties"), []byte(b.String()), 0600)
}
