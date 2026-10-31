package wizard

import "strings"

// Recognize credential-expiry diagnostics, never generic AWS failures/status 255.
// A configured SSO profile is independently checked before interactive renewal.
func ssoCredentialsExpired(message string) bool {
	lower := strings.ToLower(message)
	for _, pattern := range []string{"sso session associated", "error loading sso token", "token has expired", "token is expired", "expiredtoken"} {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return strings.Contains(lower, "getrolecredentials") && (strings.Contains(lower, "unauthorizedexception") || strings.Contains(lower, "session token not found or invalid"))
}
