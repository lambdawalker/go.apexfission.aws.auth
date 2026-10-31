package wizard

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAWSFailuresExposeStderrButNotCredentialStdout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell subprocess fixture")
	}
	root := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' '{\"SecretAccessKey\":\"stdout-private-value\"}'\nprintf '%s\\n' 'Unable to locate credentials; session_token=stderr-private-value' >&2\nexit 255\n"
	os.WriteFile(filepath.Join(root, "aws"), []byte(script), 0700)
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err := (process{}).Run(context.Background(), root, os.Environ(), "aws", "sts", "get-caller-identity")
	if err == nil || !strings.Contains(err.Error(), "Unable to locate credentials") || !strings.Contains(err.Error(), "sts get-caller-identity") {
		t.Fatalf("missing AWS diagnostic: %v", err)
	}
	if strings.Contains(err.Error(), "stdout-private-value") || strings.Contains(err.Error(), "stderr-private-value") {
		t.Fatalf("credential data exposed: %v", err)
	}
	if errors.Is(err, ErrSSOSessionExpired) {
		t.Fatal("missing generic credentials triggered SSO renewal")
	}
	_, err = (process{}).Run(context.Background(), root, os.Environ(), "aws", "sts", "get-caller-identity", "--debug")
	if err == nil || strings.Contains(err.Error(), "Unable to locate credentials") {
		t.Fatal("debug diagnostics were exposed", err)
	}
}
func TestRecognizedSSODiagnostics(t *testing.T) {
	for _, message := range []string{
		"Error loading SSO Token: Token for default does not exist",
		"The SSO session associated with this profile has expired or is otherwise invalid",
		"An error occurred (UnauthorizedException) when calling the GetRoleCredentials operation: Session token not found or invalid",
		"Error when retrieving token from sso: Token has expired and refresh failed",
	} {
		if !ssoCredentialsExpired(message) {
			t.Fatalf("SSO credential diagnostic not recognized: %s", message)
		}
	}
	for _, message := range []string{"Unable to locate credentials", "AccessDenied when calling GetBucketEncryption", "Could not connect to endpoint", "config profile missing"} {
		if ssoCredentialsExpired(message) {
			t.Fatalf("non-SSO failure classified as expired: %s", message)
		}
	}
}

func TestDefaultSSORenewalThroughCLIProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell subprocess fixture")
	}
	t.Setenv("CI", "")
	root := t.TempDir()
	ready := filepath.Join(root, "session-ready")
	script := `#!/bin/sh
case "$1 $2" in
 "sts get-caller-identity")
  if [ -f "$ATTESTRA_TEST_SESSION_READY" ]; then
   printf '%s\n' '{"Account":"123456789012"}'
  else
   printf '%s\n' 'An error occurred (UnauthorizedException) when calling the GetRoleCredentials operation: Session token not found or invalid' >&2
   exit 255
  fi;;
 "configure get")
  if [ "$3" = "sso_session" ]; then printf '%s\n' 'fixture-session'; else exit 1; fi;;
 "sso login")
  printf '%s\n' 'Open https://device.example and enter public-code'
  : > "$ATTESTRA_TEST_SESSION_READY";;
 *) exit 2;;
esac
`
	if err := os.WriteFile(filepath.Join(root, "aws"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	var output strings.Builder
	x := execution{ctx: context.Background(), dir: root, c: choices{Account: "123456789012", Region: "us-east-2"}, env: []string{"AWS_PROFILE=fixture-sso", "ATTESTRA_TEST_SESSION_READY=" + ready}, o: Options{Runner: process{}, Input: strings.NewReader(""), Output: &output}}
	if err := x.renewSSO(true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ready); err != nil || !strings.Contains(output.String(), "public-code") {
		t.Fatal("SSO renewal did not complete visibly", err)
	}
}
