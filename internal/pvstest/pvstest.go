// Package pvstest fakes a PVS for tests: its local API, as measured on a
// PVS6, and its live data websocket. It spells out the API's names itself
// rather than using package pvs's, so a wrong name in the client fails the
// tests.
package pvstest

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// Password is the fake PVS's default password, the end of its serial number.
const Password = "12345"

// Serial is the fake PVS's serial number.
const Serial = "FAKE00000" + Password

const serialVar = "/sys/info/serialnum"

// PVS is a fake local API holding a set of variables.
type PVS struct {
	*httptest.Server

	mu      sync.Mutex
	vars    map[string]string
	session string
	queries []string
}

// New serves vars, plus the serial number, until the test ends.
func New(t *testing.T, vars map[string]string) *PVS {
	p := &PVS{vars: map[string]string{serialVar: Serial}}
	maps.Copy(p.vars, vars)
	p.Server = httptest.NewTLSServer(http.HandlerFunc(p.serve))
	t.Cleanup(p.Close)
	return p
}

// Host is the address to give pvs.NewClient.
func (p *PVS) Host() string { return strings.TrimPrefix(p.URL, "https://") }

// Set changes variables, adding them if new.
func (p *PVS) Set(vars map[string]string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	maps.Copy(p.vars, vars)
}

// Delete removes variables.
func (p *PVS) Delete(names ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, n := range names {
		delete(p.vars, n)
	}
}

// Expire ends the session, as the PVS does after a while.
func (p *PVS) Expire() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.session = ""
}

// Queries returns the /vars request bodies received, oldest first, and
// forgets them.
func (p *PVS) Queries() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	q := p.queries
	p.queries = nil
	return q
}

func (p *PVS) serve(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch {
	case r.URL.Path == "/auth" && r.URL.RawQuery == "login":
		user, password, _ := r.BasicAuth()
		if user != "ssm_owner" || password != Password {
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		p.session = rand.Text()
		http.SetCookie(w, &http.Cookie{Name: "session", Value: p.session})
	case r.URL.Path == "/vars" && r.Method == http.MethodPost:
		body, _ := io.ReadAll(r.Body)
		p.queries = append(p.queries, string(body))
		p.answer(w, r, string(body))
	default:
		http.NotFound(w, r)
	}
}

func (p *PVS) answer(w http.ResponseWriter, r *http.Request, body string) {
	params := map[string]string{}
	for part := range strings.SplitSeq(body, "&") {
		k, v, _ := strings.Cut(part, "=")
		params[k] = v
	}
	if params["name"] != serialVar {
		cookie, err := r.Cookie("session")
		switch {
		case err != nil:
			http.Error(w, "", http.StatusBadRequest) // no session: 400, not 401
			return
		case cookie.Value != p.session || p.session == "":
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
	}
	out := map[string]string{}
	switch {
	case params["name"] != "":
		for name := range strings.SplitSeq(params["name"], ",") {
			v, ok := p.vars[name]
			if !ok {
				http.Error(w, "", http.StatusBadRequest)
				return
			}
			out[name] = v
		}
	case params["match"] != "":
		for name, v := range p.vars {
			if strings.Contains(name, params["match"]) {
				out[name] = v
			}
		}
	case params["set"] != "":
		name, v, _ := strings.Cut(params["set"], "=")
		if _, ok := p.vars[name]; !ok {
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		p.vars[name] = v
		out[name] = v
	}
	if len(out) == 0 {
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	_ = json.NewEncoder(w).Encode(out)
}

// Live serves a live data websocket that sends frames, one every interval, on
// each connection and then keeps it open, silent, until the test ends. It
// returns the ws:// URL.
func Live(t *testing.T, every time.Duration, frames ...string) string {
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		for _, f := range frames {
			time.Sleep(every)
			if conn.Write(context.Background(), websocket.MessageText, []byte(f)) != nil {
				return
			}
		}
		<-done
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(done) }) // runs first: cleanups are last in, first out
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}
