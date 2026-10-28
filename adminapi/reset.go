package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/lambdawalker/go.apexfission.aws.auth/envreset"
	"github.com/lambdawalker/go.apexfission.aws.auth/internal/awshttp"
	"github.com/lambdawalker/go.apexfission.aws.auth/runtimeconfig"
	"net/url"
)

type ResetClient interface {
	Invoke(context.Context, envreset.Request) (envreset.Status, error)
}
type LambdaResetClient struct {
	HTTP           awshttp.Client
	FunctionPrefix string
}

func (c LambdaResetClient) Invoke(ctx context.Context, q envreset.Request) (envreset.Status, error) {
	name, targetErr := resetFunctionName(c.FunctionPrefix, q.Environment)
	if targetErr != nil {
		return envreset.Status{}, runtimeconfig.ErrInvalid
	}
	body, e := json.Marshal(q)
	if e != nil {
		return envreset.Status{}, e
	}
	target := "https://lambda." + c.HTTP.Config.Region + ".amazonaws.com/2015-03-31/functions/" + url.PathEscape(name) + "/invocations"
	status, data, e := c.HTTP.Do(ctx, "lambda", "POST", target, body, map[string]string{"Content-Type": "application/json", "X-Amz-Invocation-Type": "RequestResponse"})
	if e != nil || status != 200 {
		return envreset.Status{}, errors.New("reset_unavailable")
	}
	var reply struct {
		Status *envreset.Status `json:"status"`
		Error  string           `json:"error"`
	}
	if json.Unmarshal(data, &reply) != nil {
		return envreset.Status{}, errors.New("reset_unavailable")
	}
	switch reply.Error {
	case "reset_disabled":
		return envreset.Status{}, runtimeconfig.ErrForbidden
	case "invalid_reset_request":
		return envreset.Status{}, runtimeconfig.ErrInvalid
	case "reset_in_progress":
		return envreset.Status{}, runtimeconfig.ErrConflict
	}
	if reply.Error != "" || reply.Status == nil || reply.Status.Environment != q.Environment {
		return envreset.Status{}, errors.New("reset_unavailable")
	}
	return *reply.Status, nil
}

func resetFunctionName(prefix, environment string) (string, error) {
	if !runtimeconfig.ValidEnvironment(environment) {
		return "", runtimeconfig.ErrInvalid
	}
	if prefix == "" {
		prefix = "attestra-reset-"
	}
	if prefix != "attestra-reset-" && prefix != "attestra-backend-reset-" {
		return "", runtimeconfig.ErrInvalid
	}
	return prefix + environment, nil
}
