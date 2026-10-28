package auth

import (
	"github.com/lambdawalker/go.apexfission.aws.auth/api"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	"github.com/lambdawalker/go.apexfission.aws.auth/email"
	"github.com/lambdawalker/go.apexfission.aws.auth/passkey"
	"github.com/lambdawalker/go.apexfission.aws.auth/router"
	"github.com/lambdawalker/go.apexfission.aws.auth/signin"
)

type Module struct {
	Email    *email.Service
	Passkeys passkey.Service
	Signin   signin.Service
	Lambda   string
}

func (Module) Describe() core.Descriptor { return core.Descriptor{ID: "auth", Version: core.Version} }
func (m Module) Routes() []router.Route {
	h := api.Handler{Service: m.Email}
	p := passkey.Handler{Service: m.Passkeys}
	target := m.Lambda
	if target == "" {
		target = "auth"
	}
	routes := []router.Route{{Method: "POST", Path: "/signup", Access: router.Public, Handler: router.HTTP(h.Signup)}, {Method: "POST", Path: "/resend", Access: router.Public, Handler: router.HTTP(h.Resend)}, {Method: "POST", Path: "/confirm", Access: router.Public, Handler: router.HTTP(h.Confirm)}, {Method: "POST", Path: "/passkeys/options", Handler: router.HTTP(p.Options)}, {Method: "POST", Path: "/passkeys/complete", Handler: router.HTTP(p.Complete)}}
	for _, path := range []string{"email/start", "email/complete", "passkey/start", "passkey/complete", "refresh", "status"} {
		s := signin.Handler{Service: m.Signin, Route: path}
		access := router.Public
		if path == "status" {
			access = router.Authenticated
		}
		routes = append(routes, router.Route{Method: "POST", Path: "/auth/" + path, Access: access, Handler: router.HTTP(s.Serve)})
	}
	for i := range routes {
		routes[i].Lambda = target
	}
	return routes
}
