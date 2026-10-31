package wizard

import (
	"os"
	"regexp"
	"sort"
	"strings"
)

var diagnosticLabels = regexp.MustCompile(`(?i)(["']?(?:password|passphrase|token|secret|proof_?key|aws_secret_access_key|aws_session_token|authorization)["']?\s*[:=]\s*)(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s,;}\]]+)`)
var diagnosticBearer = regexp.MustCompile(`(?i)\bBearer\s+[^\s"',;}]+`)
var diagnosticAccessKey = regexp.MustCompile(`\b(?:AKIA|ASIA)[A-Z0-9]{16}\b`)
var diagnosticANSI = regexp.MustCompile("\\x1b\\[[0-?]*[ -/]*[@-~]")

// pulumiFailureDetails only exposes preview/up output under Pulumi's default
// secret masking. Config/export/secret-writing commands remain suppressed.
func pulumiFailureDetails(env, args []string, b []byte) string {
	if len(args) == 0 || (args[0] != "preview" && args[0] != "up") {
		return ""
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "--show-secrets") {
			return ""
		}
	}
	secrets := []string{}
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || value == "" {
			continue
		}
		upper := strings.ToUpper(key)
		if upper == "PULUMI_CONFIG_PASSPHRASE_FILE" {
			if raw, e := os.ReadFile(value); e == nil {
				if v := strings.TrimSpace(string(raw)); v != "" {
					secrets = append(secrets, v)
				}
				clear(raw)
			}
			continue
		}
		if strings.Contains(upper, "TOKEN") || strings.Contains(upper, "SECRET") || strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "PASSPHRASE") || upper == "AWS_ACCESS_KEY_ID" {
			secrets = append(secrets, value)
		}
	}
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	text := string(b)
	for _, secret := range secrets {
		text = strings.ReplaceAll(text, secret, "[redacted]")
	}
	text = diagnosticBearer.ReplaceAllString(text, "Bearer [redacted]")
	text = diagnosticLabels.ReplaceAllString(text, "${1}[redacted]")
	text = diagnosticAccessKey.ReplaceAllString(text, "[redacted]")
	text = diagnosticANSI.ReplaceAllString(text, "")
	const max = 32 * 1024
	if len(text) > max {
		text = "[earlier Pulumi output omitted]\n" + text[len(text)-max:]
	}
	return strings.TrimSpace(text)
}
