// Package admin serves an extensible shell. Every page and API checks server grants.
package admin

import (
	"encoding/json"
	"fmt"
	"github.com/lambdawalker/go.apexfission.aws.auth/authorization"
	"github.com/lambdawalker/go.apexfission.aws.auth/config"
	"github.com/lambdawalker/go.apexfission.aws.auth/core"
	"github.com/lambdawalker/go.apexfission.aws.auth/router"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

type Page struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Permission string `json:"permission"`
	Entry      string `json:"entry"`
	Assets     fs.FS  `json:"-"`
}
type Module interface {
	core.Module
	AdminPages() []Page
}
type Shell struct {
	Login        *Login
	pages        []Page
	verifier     router.Verifier
	authorizer   authorization.Authorizer
	environments []string
	Config       *config.Service
	routes       []router.Route
}

func New(v router.Verifier, a authorization.Authorizer, environments []string) *Shell {
	return &Shell{verifier: v, authorizer: a, environments: append([]string(nil), environments...)}
}
func (s *Shell) Register(p Page) error {
	if !core.IDPattern.MatchString(p.ID) || p.Title == "" || !authorization.ValidPermission(p.Permission) || p.Assets == nil || !fs.ValidPath(p.Entry) {
		return fmt.Errorf("invalid admin page %q", p.ID)
	}
	if _, e := fs.Stat(p.Assets, p.Entry); e != nil {
		return e
	}
	for _, old := range s.pages {
		if old.ID == p.ID {
			return fmt.Errorf("duplicate admin page %s", p.ID)
		}
	}
	s.pages = append(s.pages, p)
	return nil
}
func (s *Shell) RegisterRoute(r router.Route) error {
	if !r.Admin || r.Access == router.Public || r.Permission == "" || r.Handler == nil {
		return fmt.Errorf("admin API requires protected handler")
	}
	if !strings.HasPrefix(r.Path, "/admin/api/extensions/") {
		return fmt.Errorf("admin extension API must use /admin/api/extensions/")
	}
	if e := router.Validate(r); e != nil {
		return e
	}
	for _, old := range s.routes {
		if router.Conflicts(old, r) {
			return fmt.Errorf("conflicting admin route")
		}
	}
	s.routes = append(s.routes, r)
	return nil
}
func (s *Shell) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.Method == "GET" && r.URL.Path == "/admin" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; frame-src 'self'; frame-ancestors 'none'; base-uri 'none'")
		fmt.Fprint(w, shellHTML)
		return
	}
	if r.Method == "GET" && r.URL.Path == "/admin/login-settings" {
		if s.Login == nil || s.Login.Validate() != nil {
			http.Error(w, "login not configured", 404)
			return
		}
		jsonReply(w, s.Login)
		return
	}
	if r.Method == "GET" && r.URL.Path == "/admin/shell.js" {
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, shellJS)
		return
	}
	header := strings.Fields(r.Header.Get("Authorization"))
	cookieAuth := false
	if len(header) == 0 {
		if cookie, e := r.Cookie("__Host-apex-session"); e == nil {
			header = []string{"Bearer", cookie.Value}
			cookieAuth = true
		}
	}
	if cookieAuth && r.Method != "GET" && r.Method != "HEAD" && r.Header.Get("Origin") != "https://"+r.Host {
		http.Error(w, "invalid origin", 403)
		return
	}
	if len(header) != 2 || header[0] != "Bearer" || s.verifier == nil || s.authorizer == nil {
		http.Error(w, "unauthorized", 401)
		return
	}
	p, e := s.verifier.Verify(r.Context(), header[1])
	if e != nil || p.Subject == "" {
		http.Error(w, "unauthorized", 401)
		return
	}
	env := r.URL.Query().Get("env")
	if env == "" {
		if c, e := r.Cookie("__Host-apex-env"); e == nil {
			env = c.Value
		}
	}
	if r.URL.Path == "/admin/session" && r.Method == "POST" {
		var in struct {
			Environment string `json:"environment"`
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		d.DisallowUnknownFields()
		if d.Decode(&in) != nil || d.Decode(new(any)) != io.EOF {
			http.Error(w, "invalid session", 400)
			return
		}
		known := false
		for _, v := range s.environments {
			if v == in.Environment {
				known = true
			}
		}
		if !known || s.authorizer.Authorize(r.Context(), p, in.Environment, "admin:access") != nil {
			http.Error(w, "forbidden", 403)
			return
		}
		for name, value := range map[string]string{"__Host-apex-session": header[1], "__Host-apex-env": in.Environment} {
			http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 900})
		}
		w.WriteHeader(204)
		return
	}
	known := false
	for _, v := range s.environments {
		if v == env {
			known = true
		}
	}
	if r.URL.Path == "/admin/api/environments" && r.Method == "GET" {
		allowed := []string{}
		for _, env := range s.environments {
			if s.authorizer.Authorize(r.Context(), p, env, "admin:access") == nil {
				allowed = append(allowed, env)
			}
		}
		jsonReply(w, allowed)
		return
	}
	if !known {
		http.Error(w, "unknown environment", 400)
		return
	}
	if r.URL.Path == "/admin/api/navigation" && r.Method == "GET" {
		pages := []Page{}
		for _, page := range s.pages {
			if s.authorizer.Authorize(r.Context(), p, env, page.Permission) == nil {
				pages = append(pages, page)
			}
		}
		jsonReply(w, pages)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/admin/pages/") && r.Method == "GET" {
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/admin/pages/"), "/", 2)
		if len(parts) != 2 || !fs.ValidPath(parts[1]) {
			http.NotFound(w, r)
			return
		}
		for _, page := range s.pages {
			if page.ID != parts[0] {
				continue
			}
			if s.authorizer.Authorize(r.Context(), p, env, page.Permission) != nil {
				http.Error(w, "forbidden", 403)
				return
			}
			b, e := fs.ReadFile(page.Assets, parts[1])
			if e != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(parts[1])))
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; object-src 'none'; base-uri 'none'")
			w.Write(b)
			return
		}
	}
	if strings.HasPrefix(r.URL.Path, "/admin/api/config/") && s.Config != nil {
		ns := strings.TrimPrefix(r.URL.Path, "/admin/api/config/")
		if !core.IDPattern.MatchString(ns) {
			http.NotFound(w, r)
			return
		}
		perm := ns + ":config.read"
		if r.Method == "POST" {
			perm = ns + ":config.write"
		}
		if s.authorizer.Authorize(r.Context(), p, env, perm) != nil {
			http.Error(w, "forbidden", 403)
			return
		}
		if r.Method == "GET" {
			if r.URL.Query().Get("history") == "true" {
				rows, e := s.Config.Store.History(r.Context(), env, ns)
				if e != nil {
					http.Error(w, "unavailable", 503)
					return
				}
				jsonReply(w, rows)
				return
			}
			row, e := s.Config.Store.Get(r.Context(), env, ns)
			if e != nil {
				http.Error(w, "not found", 404)
				return
			}
			jsonReply(w, row)
			return
		}
		if r.Method == "POST" {
			var in struct {
				Expected int64           `json:"expected_revision"`
				Version  int             `json:"schema_version"`
				Value    json.RawMessage `json:"value"`
			}
			d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128*1024))
			d.DisallowUnknownFields()
			if d.Decode(&in) != nil || d.Decode(new(any)) != io.EOF {
				http.Error(w, "invalid request", 400)
				return
			}
			out, e := s.Config.Publish(r.Context(), p, env, ns, in.Expected, in.Version, in.Value)
			if e != nil {
				status := 400
				if e == config.ErrConflict {
					status = 409
				}
				if e == authorization.ErrDenied {
					status = 403
				}
				http.Error(w, "publication rejected", status)
				return
			}
			jsonReply(w, out)
			return
		}
	}
	rr := router.New(s.verifier, s.authorizer, env)
	for _, route := range s.routes {
		if e := rr.Register(route); e != nil {
			http.Error(w, "configuration error", 500)
			return
		}
	}
	rr.ServeHTTP(w, r)
}
func jsonReply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

const shellHTML = `<!doctype html><html><head><meta charset="utf-8"><title>Application administration</title></head><body><h1>Administration</h1><button id="login">Sign in</button><p>Sign in through the dedicated administrator identity provider, or use an administrator access token.</p><label>Admin access token <input id="token" type="password" autocomplete="off"></label><button id="connect">Connect</button><label>Environment <select id="env"></select></label><nav id="nav"></nav><main id="page"></main><section><h2>Module configuration</h2><label>Namespace <input id="namespace" value="sales"></label><button id="load-config">Load</button><button id="history">History</button><label>Schema version <input id="schema" type="number" value="1"></label><textarea id="config-value" rows="8" cols="60">{}</textarea><button id="publish">Publish revision</button><pre id="config-history"></pre></section><p id="status" role="status"></p><script src="/admin/shell.js"></script></body></html>`
const shellJS = `let token='';const el=id=>document.getElementById(id);async function get(p){const r=await fetch(p,{headers:token?{Authorization:'Bearer '+token}:{}});if(!r.ok)throw Error('Request rejected: '+r.status);return r}async function session(env){const r=await fetch('/admin/session',{method:'POST',headers:{'Content-Type':'application/json',...(token?{Authorization:'Bearer '+token}:{})},body:JSON.stringify({environment:env})});if(!r.ok)throw Error('Session rejected: '+r.status)}async function nav(){el('page').replaceChildren();const env=el('env').value;await session(env);const pages=await(await get('/admin/api/navigation?env='+encodeURIComponent(env))).json();el('nav').replaceChildren();for(const p of pages){const b=document.createElement('button');b.textContent=p.title;b.onclick=()=>{const frame=document.createElement('iframe');frame.title=p.title;frame.src='/admin/pages/'+p.id+'/'+p.entry+'?env='+encodeURIComponent(env);el('page').replaceChildren(frame)};el('nav').append(b)}}el('connect').onclick=async()=>{try{token=el('token').value;el('token').value='';const envs=await(await get('/admin/api/environments')).json();el('env').replaceChildren(...envs.map(e=>new Option(e,e)));await nav();token=''}catch(e){el('status').textContent=e.message}};el('env').onchange=()=>{revision=0;el('config-value').value='{}';el('config-history').textContent='';nav().catch(e=>el('status').textContent=e.message)};
let revision=0;const configURL=()=>'/admin/api/config/'+encodeURIComponent(el('namespace').value)+'?env='+encodeURIComponent(el('env').value);el('load-config').onclick=async()=>{try{const r=await(await get(configURL())).json();revision=r.revision;el('schema').value=r.schema_version;el('config-value').value=JSON.stringify(r.value,null,2);el('status').textContent='Loaded revision '+revision}catch(e){revision=0;el('status').textContent=e.message+'; a new namespace starts at revision 0'}};el('history').onclick=async()=>{try{el('config-history').textContent=JSON.stringify(await(await get(configURL()+'&history=true')).json(),null,2)}catch(e){el('status').textContent=e.message}};el('publish').onclick=async()=>{try{const value=JSON.parse(el('config-value').value);if(!confirm('Publish '+el('namespace').value+' in '+el('env').value+' from revision '+revision+'?'))return;const r=await fetch(configURL(),{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({expected_revision:revision,schema_version:Number(el('schema').value),value})});if(!r.ok)throw Error('Publication rejected: '+r.status);revision=(await r.json()).revision;el('status').textContent='Published revision '+revision}catch(e){el('status').textContent=e.message}};
const b64=b=>btoa(String.fromCharCode(...new Uint8Array(b))).replaceAll('+','-').replaceAll('/','_').replaceAll('=','');async function settings(){const r=await fetch('/admin/login-settings');if(!r.ok)throw Error('Managed login is not configured');return r.json()}el('login').onclick=async()=>{try{const s=await settings();if(new URL(s.redirect_uri).origin!==location.origin)throw Error('Open the configured admin origin to sign in');const verifier=b64(crypto.getRandomValues(new Uint8Array(32))),state=b64(crypto.getRandomValues(new Uint8Array(24)));sessionStorage.setItem('apex-pkce',verifier);sessionStorage.setItem('apex-state',state);const challenge=b64(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(verifier)));const u=new URL(s.domain+'/oauth2/authorize');u.search=new URLSearchParams({response_type:'code',client_id:s.client_id,redirect_uri:s.redirect_uri,scope:'openid email aws.cognito.signin.user.admin',state,code_challenge:challenge,code_challenge_method:'S256'});location.assign(u)}catch(e){el('status').textContent=e.message}};(async()=>{const params=new URLSearchParams(location.search);if(!params.has('code'))return;try{const expected=sessionStorage.getItem('apex-state'),verifier=sessionStorage.getItem('apex-pkce');sessionStorage.removeItem('apex-state');sessionStorage.removeItem('apex-pkce');history.replaceState({},'',location.pathname);if(!expected||params.get('state')!==expected||!verifier)throw Error('Invalid login state');const s=await settings();const r=await fetch(s.domain+'/oauth2/token',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},body:new URLSearchParams({grant_type:'authorization_code',client_id:s.client_id,redirect_uri:s.redirect_uri,code:params.get('code'),code_verifier:verifier})});if(!r.ok)throw Error('Login exchange failed');el('token').value=(await r.json()).access_token;await el('connect').onclick()}catch(e){el('status').textContent=e.message}})();
`
