package wizard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	transport "github.com/lambdawalker/go.apexfission.aws.auth/deploy/registry"
	model "github.com/lambdawalker/go.apexfission.aws.auth/registry"
	"github.com/lambdawalker/go.apexfission.aws.auth/runtimeconfig"
)

// RegistryChanger performs authenticated registry operations. Production uses the
// existing SigV4 client with refreshed credentials from the selected AWS CLI profile.
type RegistryChanger interface {
	Change(model.Change) (model.Receipt, error)
}
type RegistryFactory func(context.Context, string, string, string) (RegistryChanger, error)
type publication struct {
	JournalVersion                                        int
	Desired                                               choices
	Outputs, Shared                                       map[string]any
	DNSDone, AdminDone                                    bool
	Identity, Stack, Account, Region, Backend, URL, Stage string
	Receipt                                               model.Receipt
	Config                                                model.Configuration
	Defaults                                              *runtimeconfig.Config
}
type cliCredentials struct{ x *execution }

func (p cliCredentials) Retrieve(context.Context) (aws.Credentials, error) {
	b, e := p.x.run("aws", "configure", "export-credentials", "--format", "process")
	if e != nil {
		return aws.Credentials{}, errors.New("AWS CLI cannot export session credentials; refresh your selected profile/SSO session")
	}
	var c struct{ AccessKeyId, SecretAccessKey, SessionToken string }
	if json.Unmarshal(b, &c) != nil || c.AccessKeyId == "" || c.SecretAccessKey == "" {
		return aws.Credentials{}, errors.New("invalid AWS CLI credential export (suppressed)")
	}
	return aws.Credentials{AccessKeyID: c.AccessKeyId, SecretAccessKey: c.SecretAccessKey, SessionToken: c.SessionToken, Source: "wizard AWS CLI"}, nil
}
func (x *execution) registry(address string) (RegistryChanger, error) {
	parsed, err := url.Parse(address)
	pattern := regexp.MustCompile(`^[a-z0-9]+\.execute-api\.` + regexp.QuoteMeta(x.c.Region) + `\.amazonaws\.com$`)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || !pattern.MatchString(parsed.Host) {
		return nil, errors.New("registry URL must be the raw API Gateway HTTPS endpoint in the selected region, without path, port, query or credentials")
	}

	if x.o.Registry != nil {
		return x.o.Registry(x.ctx, address, x.c.Region, x.c.stack(false))
	}
	if !strings.HasPrefix(address, "https://") {
		return nil, errors.New("shared registry API URL must be HTTPS")
	}
	return &transport.Client{URL: address, Region: x.c.Region, Environment: x.c.stack(false), Provider: cliCredentials{x}, HTTP: x.o.HTTP}, nil
}
func (x *execution) publicationLocation() (string, string) {
	u, _ := url.Parse(x.c.backend())
	return u.Host, strings.TrimPrefix(u.Path, "/") + "/receipts/" + x.c.stack(false) + ".json"
}
func (x *execution) storePublication(p *publication) error {
	if e := save(x.receiptPath("publication"), p); e != nil {
		return e
	}
	bucket, key := x.publicationLocation()
	return x.aws(nil, "s3api", "put-object", "--bucket", bucket, "--key", key, "--expected-bucket-owner", x.c.Account, "--body", x.receiptPath("publication"))
}
func (x *execution) loadPublication() (*publication, error) {
	bucket, key := x.publicationLocation()
	var list struct{ Contents []struct{ Key string } }
	if e := x.aws(&list, "s3api", "list-objects-v2", "--bucket", bucket, "--prefix", key, "--expected-bucket-owner", x.c.Account); e != nil {
		return nil, e
	}
	exists := false
	for _, v := range list.Contents {
		if v.Key == key {
			exists = true
		}
	}
	if !exists {
		return nil, nil
	}
	f, e := os.CreateTemp("", "attestra-receipt-*")
	if e != nil {
		return nil, e
	}
	name := f.Name()
	f.Close()
	defer os.Remove(name)
	if e = x.aws(nil, "s3api", "get-object", "--bucket", bucket, "--key", key, "--expected-bucket-owner", x.c.Account, name); e != nil {
		return nil, e
	}
	b, e := os.ReadFile(name)
	if e != nil {
		return nil, e
	}
	var p publication
	if json.Unmarshal(b, &p) != nil {
		return nil, errors.New("invalid durable registry receipt")
	}
	if p.Identity != x.c.identity() || p.Stack != x.c.stack(false) || p.Account != x.c.Account || p.Region != x.c.Region || p.Backend != x.c.backend() {
		return nil, errors.New("registry receipt target mismatch")
	}
	return &p, nil
}
func (x *execution) beginPublication(shared map[string]any) (*publication, error) {
	p, e := x.loadPublication()
	if e != nil {
		return nil, e
	}
	if p != nil && p.Stage != "complete" {
		if p.Stage == "postapply" || p.Stage == "ready" || p.Stage == "defaults" {
			return nil, errors.New("registry publication pending: use --operation publish to resume without rerunning infrastructure")
		}
		if p.URL != str(shared, "apiUrl") {
			return nil, errors.New("pending registry endpoint mismatch")
		}
	} else {
		token := make([]byte, 24)
		if _, e = rand.Read(token); e != nil {
			return nil, e
		}
		p = &publication{Identity: x.c.identity(), Stack: x.stack, Account: x.c.Account, Region: x.c.Region, Backend: x.c.backend(), URL: str(shared, "apiUrl"), Stage: "begin", Receipt: model.Receipt{Token: hex.EncodeToString(token)}}
		if e = x.storePublication(p); e != nil {
			return nil, e
		}
	}
	client, e := x.registry(p.URL)
	if e != nil {
		return nil, e
	}
	receipt, e := client.Change(model.Change{Operation: "begin", Token: p.Receipt.Token})
	if e != nil {
		return nil, e
	}
	p.Receipt = receipt
	p.Stage = "updating"
	if e = x.storePublication(p); e != nil {
		return nil, e
	}
	return p, nil
}
func (x *execution) finishPublication(p *publication, out map[string]any) error {
	p.Config = model.Configuration{APIURL: str(out, "apiUrl"), AWSRegion: x.c.Region, CognitoUserPoolID: str(out, "userPoolId"), CognitoClientID: str(out, "clientId")}
	b, _ := json.Marshal(out["runtimeDefaults"])
	var defaults runtimeconfig.Config
	if json.Unmarshal(b, &defaults) != nil || defaults.Environment != x.c.stack(false) || defaults.Validate() != nil {
		return errors.New("backend runtimeDefaults output invalid or mismatched; no registry publication performed")
	}
	p.Defaults = &defaults
	p.Config.Features.IDCapture = defaults.Features.IDCapture
	if e := model.Validate(p.Config); e != nil {
		return e
	}
	p.Stage = "ready"
	if e := x.storePublication(p); e != nil {
		return e
	}
	return x.publish(p)
}
func (x *execution) publish(p *publication) error {
	if p.JournalVersion != 1 || !p.DNSDone || !p.AdminDone || p.Defaults == nil || p.Defaults.Environment != x.c.stack(false) || p.Defaults.Validate() != nil || model.Validate(p.Config) != nil {
		return errors.New("publication receipt lacks completed deployment prerequisites or valid output configuration; recover the original deployment before publication")
	}

	client, e := x.registry(p.URL)
	if e != nil {
		return e
	}
	if p.Stage == "ready" {
		if _, e = client.Change(model.Change{Operation: "defaults", Token: p.Receipt.Token, Revision: p.Receipt.Revision, Defaults: p.Defaults}); e != nil {
			return fmt.Errorf("runtime defaults publication pending; retry --operation publish: %w", e)
		}
		p.Stage = "defaults"
		if e = x.storePublication(p); e != nil {
			return e
		}
	}
	if p.Stage != "defaults" {
		return errors.New("receipt is not ready for publication")
	}
	if _, e = client.Change(model.Change{Operation: "publish", Token: p.Receipt.Token, Revision: p.Receipt.Revision, Config: p.Config}); e != nil {
		return fmt.Errorf("registry publication pending; retry --operation publish: %w", e)
	}
	p.Stage = "complete"
	return x.storePublication(p)
}
func (x *execution) resumePublication() error {
	release, e := x.acquireLock()
	if e != nil {
		return e
	}
	defer release()
	p, e := x.loadPublication()
	if e != nil {
		return e
	}
	if p == nil {
		return errors.New("no saved publication receipt")
	}
	return x.completeApplied(p)
}
