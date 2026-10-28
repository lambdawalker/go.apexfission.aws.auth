package admin

import (
	"fmt"
	"net/url"
)

// Login configures Cognito authorization-code + PKCE browser sign-in.
// The callback origin must be hosted on this shell's HTTPS origin.
type Login struct {
	Domain      string `json:"domain"`
	ClientID    string `json:"client_id"`
	RedirectURI string `json:"redirect_uri"`
}

func (l Login) Validate() error {
	for _, raw := range []string{l.Domain, l.RedirectURI} {
		u, e := url.Parse(raw)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
			return fmt.Errorf("login requires trusted HTTPS URLs")
		}
	}
	if l.ClientID == "" {
		return fmt.Errorf("login client required")
	}
	return nil
}
