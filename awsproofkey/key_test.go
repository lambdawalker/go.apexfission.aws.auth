package awsproofkey

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:proof-key-AbCdEf"
const version = "01234567-89ab-cdef-0123-456789abcdef"

type fakeSecrets struct {
	calls  int
	input  *secretsmanager.GetSecretValueInput
	output *secretsmanager.GetSecretValueOutput
	err    error
}

func (f *fakeSecrets) GetSecretValue(_ context.Context, in *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	f.calls++
	f.input = in
	return f.output, f.err
}

// These tests catch identity changes, unbound reads, and accidental error leakage.
func TestLoadPreservesKeyAndPinsVersion(t *testing.T) {
	want := []byte("0123456789abcdef0123456789abcdef")
	client := &fakeSecrets{output: &secretsmanager.GetSecretValueOutput{ARN: aws.String(secretARN), VersionId: aws.String(version), SecretString: aws.String(base64.StdEncoding.EncodeToString(want))}}
	key, err := Load(context.Background(), client, secretARN, version)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(key, want) {
		t.Fatal("key identity changed")
	}
	if client.calls != 1 || aws.ToString(client.input.SecretId) != secretARN || aws.ToString(client.input.VersionId) != version {
		t.Fatalf("read was not bound to configured secret/version: %#v", client.input)
	}
}

func TestLoadWithoutVersionUsesCurrentSecret(t *testing.T) {
	client := &fakeSecrets{output: &secretsmanager.GetSecretValueOutput{SecretString: aws.String(base64.StdEncoding.EncodeToString(make([]byte, 32)))}}
	if _, err := Load(context.Background(), client, secretARN, ""); err != nil {
		t.Fatal(err)
	}
	if client.input.VersionId != nil {
		t.Fatal("optional version should remain unset")
	}
}

func TestLoadRejectsInvalidBindingBeforeNetwork(t *testing.T) {
	for _, tc := range []struct{ name, arn, version string }{
		{"missing ARN", "", ""}, {"secret name", "proof-key", ""},
		{"wrong service", "arn:aws:ssm:us-east-1:123456789012:secret:proof-key", ""},
		{"missing region", "arn:aws:secretsmanager::123456789012:secret:proof-key", ""},
		{"wrong resource", "arn:aws:secretsmanager:us-east-1:123456789012:parameter:proof-key", ""},
		{"invalid version", secretARN, "bad version"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeSecrets{}
			if _, err := Load(context.Background(), client, tc.arn, tc.version); err == nil {
				t.Fatal("accepted invalid binding")
			}
			if client.calls != 0 {
				t.Fatal("invalid binding reached AWS")
			}
		})
	}
	if _, err := Load(context.Background(), nil, secretARN, ""); err == nil {
		t.Fatal("accepted missing client")
	}
}

func TestLoadRejectsInvalidValueAndRedactsErrors(t *testing.T) {
	secret := "TOP-SECRET-value-that-must-never-be-logged"
	for _, tc := range []struct {
		name    string
		out     *secretsmanager.GetSecretValueOutput
		err     error
		version string
	}{
		{name: "AWS failure", err: errors.New(secret)},
		{name: "nil response"},
		{name: "empty response", out: &secretsmanager.GetSecretValueOutput{}},
		{name: "binary unsupported", out: &secretsmanager.GetSecretValueOutput{SecretBinary: make([]byte, 32)}},
		{name: "invalid base64", out: &secretsmanager.GetSecretValueOutput{SecretString: aws.String(secret)}},
		{name: "short key", out: &secretsmanager.GetSecretValueOutput{SecretString: aws.String(base64.StdEncoding.EncodeToString([]byte(secret[:31])))}},
		{name: "empty key", out: &secretsmanager.GetSecretValueOutput{SecretString: aws.String("")}},
		{name: "version mismatch", version: version, out: &secretsmanager.GetSecretValueOutput{SecretString: aws.String(base64.StdEncoding.EncodeToString(make([]byte, 32))), VersionId: aws.String("ffffffff-ffff-ffff-ffff-ffffffffffff")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key, err := Load(context.Background(), &fakeSecrets{output: tc.out, err: tc.err}, secretARN, tc.version)
			if err == nil || key != nil {
				t.Fatal("accepted invalid secret")
			}
			if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), base64.StdEncoding.EncodeToString([]byte(secret[:31]))) {
				t.Fatal("error disclosed secret")
			}
		})
	}
}
