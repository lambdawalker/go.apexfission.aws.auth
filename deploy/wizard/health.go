package wizard

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	model "github.com/lambdawalker/go.apexfission.aws.auth/registry"
)

func (x *execution) health(out map[string]any) error {
	if x.project == "attestra-backend" {
		configuration := model.Configuration{APIURL: str(out, "apiUrl"), AWSRegion: x.c.Region, CognitoUserPoolID: str(out, "userPoolId"), CognitoClientID: str(out, "clientId")}
		if err := model.Validate(configuration); err != nil {
			return errors.New("backend public API/Cognito output configuration is incomplete")
		}
	}
	if x.project == "attestra-shared" && (str(out, "indexUrl") == "" || str(out, "adminUrl") == "") {
		return errors.New("shared index/admin HTTPS outputs are incomplete")
	}

	if x.project == "attestra-backend" && str(out, "assetLinksUrl") == "" {
		if x.c.AndroidAssociations != "" {
			return errors.New("configured Android association hosting is missing")
		}
		fmt.Fprintln(x.o.Output, "WARN: Android app links/passkey association hosting is not configured; use --android-associations with the Android package/fingerprints. HTTPS checks do not verify app links or a complete sign-in flow.")
	}

	for _, key := range []string{"apiUrl", "indexUrl", "adminUrl", "assetLinksUrl"} {
		address := str(out, key)
		if address == "" {
			continue
		}
		method := "GET"
		if key == "apiUrl" && x.project != "attestra-shared" {
			method = "POST"
			address = strings.TrimRight(address, "/") + "/auth/status"
		} else if key == "apiUrl" {
			continue
		}
		u, e := url.Parse(address)
		if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("invalid HTTPS health URL for %s", key)
		}
		req, e := http.NewRequestWithContext(x.ctx, method, address, nil)
		if e != nil {
			return e
		}
		resp, e := x.o.HTTP.Do(req)
		if e != nil {
			return fmt.Errorf("HTTPS health check %s failed", key)
		}
		body, e := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
		resp.Body.Close()
		if e != nil || len(body) > 1024*1024 {
			return fmt.Errorf("invalid health response for %s", key)
		}
		ok := false
		switch key {
		case "apiUrl":
			var status struct{ Error string }
			ok = resp.StatusCode == 401 && json.Unmarshal(body, &status) == nil && status.Error == "sign_in_required"
		case "indexUrl":
			var listing struct {
				SchemaVersion int           `json:"schemaVersion"`
				Environments  []model.Entry `json:"environments"`
			}
			ok = resp.StatusCode == 200 && json.Unmarshal(body, &listing) == nil && listing.SchemaVersion == 1 && listing.Environments != nil
			if ok && x.project == "attestra-backend" {
				ok = false
				for _, entry := range listing.Environments {
					if entry.ID == x.c.stack(false) && entry.Config.APIURL == str(out, "apiUrl") && entry.Config.CognitoUserPoolID == str(out, "userPoolId") && entry.Config.CognitoClientID == str(out, "clientId") && entry.Config.AWSRegion == x.c.Region && entry.ConfigHash == model.Hash(entry.Config) {
						ok = true
					}
				}
			}
		case "assetLinksUrl":
			var actual []json.RawMessage
			ok = resp.StatusCode == 200 && json.Unmarshal(body, &actual) == nil && actual != nil
			if ok && str(out, "assetLinksContent") != "" {
				var expected any
				var actualValue any
				ok = json.Unmarshal([]byte(str(out, "assetLinksContent")), &expected) == nil && json.Unmarshal(body, &actualValue) == nil
				if ok {
					a, _ := json.Marshal(actualValue)
					b, _ := json.Marshal(expected)
					ok = string(a) == string(b)
				}
			}
		case "adminUrl":
			ok = resp.StatusCode == 200 || resp.StatusCode == 302 || resp.StatusCode == 303
			if ok && resp.StatusCode != 200 {
				location, e := url.Parse(resp.Header.Get("Location"))
				login, e2 := url.Parse(str(out, "adminLoginDomain"))
				ok = e == nil && e2 == nil && location.Scheme == "https" && location.Hostname() == login.Hostname() && login.Hostname() != ""
			}
		}
		fmt.Fprintf(x.o.Output, "%s: HTTP %d\n", key, resp.StatusCode)
		if !ok {
			return fmt.Errorf("HTTPS health check %s returned an unexpected status or payload", key)
		}
	}
	if str(out, "apiUrl") == "" && str(out, "indexUrl") == "" {
		return errors.New("deployed HTTPS endpoint outputs are missing")
	}
	return nil
}
