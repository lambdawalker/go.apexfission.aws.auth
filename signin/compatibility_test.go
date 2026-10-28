package signin

import (
	"context"
	"github.com/aws/aws-lambda-go/events"
	"testing"
)

func TestStatusWithoutEnrollmentRetainsBasicPasskeyContract(t *testing.T) {
	h := Handler{Service: Service{Client: &fake{}, ClientID: "client"}, Route: "status"}
	out, _ := h.Serve(context.Background(), events.APIGatewayV2HTTPRequest{RawPath: "/auth/status", RequestContext: events.APIGatewayV2HTTPRequestContext{HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{Method: "POST"}}, Body: "{}", Headers: map[string]string{"authorization": "Bearer access"}})
	if out.StatusCode != 200 || out.Body != `{"has_passkey":false}` {
		t.Fatal(out)
	}
}
