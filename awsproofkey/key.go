// Package awsproofkey loads the existing email proof key during service startup.
package awsproofkey

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// GetSecretValueAPI is the Secrets Manager operation required by Load.
type GetSecretValueAPI interface {
	GetSecretValue(context.Context, *secretsmanager.GetSecretValueInput, ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// Load reads a base64 SecretString from an explicitly bound secret ARN. A version
// pins the read to that immutable version; otherwise AWS selects AWSCURRENT.
// Call once during startup and retain the returned bytes for the service lifetime.
// No fallback, key generation, rotation, or process-global caching is performed.
// Errors deliberately omit AWS response details and secret material.
func Load(ctx context.Context, client GetSecretValueAPI, secretARN, version string) ([]byte, error) {
	parsed, err := arn.Parse(secretARN)
	if err != nil || parsed.Partition == "" || parsed.Service != "secretsmanager" || parsed.Region == "" || len(parsed.AccountID) != 12 || !strings.HasPrefix(parsed.Resource, "secret:") || len(parsed.Resource) <= len("secret:") || strings.ContainsAny(secretARN, " \t\r\n") {
		return nil, errors.New("invalid proof-key secret ARN")
	}
	for _, c := range parsed.AccountID {
		if c < '0' || c > '9' {
			return nil, errors.New("invalid proof-key secret ARN")
		}
	}
	if version != "" {
		if len(version) < 32 || len(version) > 64 {
			return nil, errors.New("invalid proof-key secret version")
		}
		for _, c := range version {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return nil, errors.New("invalid proof-key secret version")
			}
		}
	}
	if client == nil {
		return nil, errors.New("proof-key secret client unavailable")
	}
	input := &secretsmanager.GetSecretValueInput{SecretId: aws.String(secretARN)}
	if version != "" {
		input.VersionId = aws.String(version)
	}
	output, err := client.GetSecretValue(ctx, input)
	if err != nil {
		return nil, errors.New("proof-key secret unavailable")
	}
	if output == nil || output.SecretString == nil {
		return nil, errors.New("proof-key secret must contain a base64 SecretString")
	}
	if version != "" && aws.ToString(output.VersionId) != version {
		return nil, errors.New("proof-key secret version mismatch")
	}
	key, err := base64.StdEncoding.DecodeString(*output.SecretString)
	if err != nil || len(key) < 32 {
		return nil, errors.New("invalid proof-key secret value")
	}
	return key, nil
}
