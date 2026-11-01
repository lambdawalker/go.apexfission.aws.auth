package bootstrap

import (
	"bytes"
	"context"
	"encoding/base64"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type startupSecrets struct{ calls int }

func (s *startupSecrets) GetSecretValue(context.Context, *secretsmanager.GetSecretValueInput, ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	s.calls++
	return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))}, nil
}
func TestServiceLoadsSecretOnceAtInitialization(t *testing.T) {
	env := map[string]string{"TABLE_NAME": "table", "POOL_ID": "pool", "CLIENT_ID": "client", "APP_ORIGIN": "https://example.test", "SENDER_ADDRESS": "sender@example.test", "PROOF_KEY_SECRET_ARN": "arn:aws:secretsmanager:us-east-1:123456789012:secret:proof-key-AbCdEf", "PROOF_KEY": "invalid legacy key"}
	client := &startupSecrets{}
	service, err := newService(context.Background(), aws.Config{}, func(k string) string { return env[k] }, client)
	if err != nil {
		t.Fatal(err)
	}
	if client.calls != 1 {
		t.Fatalf("startup fetched secret %d times", client.calls)
	}
	for range 3 {
		if !bytes.Equal(service.Key, []byte("0123456789abcdef0123456789abcdef")) {
			t.Fatal("service lost loaded key")
		}
	}
	if client.calls != 1 {
		t.Fatal("service reused network reader")
	}
}
func TestServiceRejectsPlaintextOnlyConfiguration(t *testing.T) {
	client := &startupSecrets{}
	_, err := newService(context.Background(), aws.Config{}, func(string) string { return "" }, client)
	if err == nil {
		t.Fatal("accepted missing configuration")
	}
	if client.calls != 0 {
		t.Fatal("incomplete config reached AWS")
	}
	env := map[string]string{"TABLE_NAME": "table", "POOL_ID": "pool", "CLIENT_ID": "client", "APP_ORIGIN": "https://example.test", "SENDER_ADDRESS": "sender@example.test", "PROOF_KEY": base64.StdEncoding.EncodeToString(make([]byte, 32))}
	if _, err := newService(context.Background(), aws.Config{}, func(k string) string { return env[k] }, client); err == nil {
		t.Fatal("accepted legacy plaintext key")
	}
	if client.calls != 0 {
		t.Fatal("missing ARN reached AWS")
	}
}
