package router

import (
	"context"
	"encoding/base64"
	"github.com/aws/aws-lambda-go/events"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

type LambdaHandler func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)

// HTTP adapts existing Attestra Lambda handlers without changing their wire contract.
func HTTP(h LambdaHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, e := io.ReadAll(io.LimitReader(r.Body, 65537))
		if e != nil || len(b) > 65536 {
			http.Error(w, "invalid body", 400)
			return
		}
		headers := map[string]string{}
		for k, v := range r.Header {
			headers[strings.ToLower(k)] = strings.Join(v, ",")
		}
		req := events.APIGatewayV2HTTPRequest{Version: "2.0", RawPath: r.URL.Path, RawQueryString: r.URL.RawQuery, Headers: headers, Body: string(b)}
		req.RequestContext.HTTP.Method = r.Method
		req.RequestContext.HTTP.SourceIP = r.RemoteAddr
		out, e := h(r.Context(), req)
		if e != nil {
			http.Error(w, "unavailable", 503)
			return
		}
		for k, v := range out.Headers {
			w.Header().Set(k, v)
		}
		for _, c := range out.Cookies {
			w.Header().Add("Set-Cookie", c)
		}
		data := []byte(out.Body)
		if out.IsBase64Encoded {
			data, e = base64.StdEncoding.DecodeString(out.Body)
			if e != nil {
				http.Error(w, "unavailable", 503)
				return
			}
		}
		w.WriteHeader(out.StatusCode)
		w.Write(data)
	})
}

// Lambda exposes an HTTP application through API Gateway payload version 2.
func Lambda(h http.Handler) LambdaHandler {
	return func(ctx context.Context, in events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		body := []byte(in.Body)
		if in.IsBase64Encoded {
			var e error
			body, e = base64.StdEncoding.DecodeString(in.Body)
			if e != nil {
				return events.APIGatewayV2HTTPResponse{StatusCode: 400}, nil
			}
		}
		target := in.RawPath
		if in.RawQueryString != "" {
			target += "?" + in.RawQueryString
		}
		req, e := http.NewRequestWithContext(ctx, in.RequestContext.HTTP.Method, target, strings.NewReader(string(body)))
		if e != nil {
			return events.APIGatewayV2HTTPResponse{StatusCode: 400}, nil
		}
		for k, v := range in.Headers {
			req.Header.Set(k, v)
		}
		for _, c := range in.Cookies {
			req.Header.Add("Cookie", c)
		}
		req.Host = in.RequestContext.DomainName
		req.RemoteAddr = in.RequestContext.HTTP.SourceIP
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		headers := map[string]string{}
		for k, v := range w.Header() {
			if k != "Set-Cookie" {
				headers[k] = strings.Join(v, ",")
			}
		}
		return events.APIGatewayV2HTTPResponse{StatusCode: w.Code, Headers: headers, Cookies: w.Header().Values("Set-Cookie"), Body: base64.StdEncoding.EncodeToString(w.Body.Bytes()), IsBase64Encoded: true}, nil
	}
}
