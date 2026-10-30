package guard

import (
	"crypto/sha256"
	"encoding/base64"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// CodeHash uses AWS Lambda's base64 SHA-256 of the actual ZIP bytes. It is not
// the source-input cache key; configuration and IAM remain independently planned.
func CodeHash(path string) pulumi.StringOutput {
	return pulumi.ToOutput(path).ApplyT(codeHash).(pulumi.StringOutput)
}
func codeHash(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return base64.StdEncoding.EncodeToString(sum[:]), nil
}
