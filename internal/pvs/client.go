// Package pvs talks to a SunStrong PVS5 or PVS6 over its local API.
package pvs

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ErrBadRequest is the PVS's answer to an unknown variable name, and to a
// match that finds nothing.
var ErrBadRequest = errors.New("PVS answered 400 Bad Request")

var errExpired = errors.New("PVS session expired")

// Client reads varserver variables. It logs in before its first request and
// again when the session expires.
type Client struct {
	base     string
	http     *http.Client
	requests map[string]*requestCount

	mu       sync.Mutex
	password string
	loggedIn bool
}

// requestKinds are the requests the client counts: logins and the three
// /vars queries.
var requestKinds = []string{"login", "name", "match", "set"}

type requestCount struct{ total, failures atomic.Uint64 }

// RequestCount is how many requests of one kind the client has made, and how
// many failed. The PVS's 400 for an unknown name or an empty match, and its 401
// for an expired session, are answers, not failures.
type RequestCount struct {
	Kind            string
	Total, Failures uint64
}

// Requests returns the count for each kind of request.
func (c *Client) Requests() []RequestCount {
	var out []RequestCount
	for _, kind := range requestKinds {
		n := c.requests[kind]
		out = append(out, RequestCount{kind, n.total.Load(), n.failures.Load()})
	}
	return out
}

func (c *Client) count(kind string, failed bool) {
	n := c.requests[kind]
	n.total.Add(1)
	if failed {
		n.failures.Add(1)
	}
}

// NewClient returns a client for the PVS at host. An empty password is
// derived from the serial number, as the PVS does by default.
func NewClient(host, password string, timeout time.Duration) *Client {
	jar, _ := cookiejar.New(nil) // fails only for a bad public suffix list
	requests := map[string]*requestCount{}
	for _, kind := range requestKinds {
		requests[kind] = &requestCount{}
	}
	return &Client{
		base:     "https://" + host,
		password: password,
		requests: requests,
		http: &http.Client{
			Jar:       jar,
			Timeout:   timeout,
			Transport: selfSignedTransport(),
		},
	}
}

// selfSignedTransport accepts any certificate, since the PVS's is self-signed.
// CodeQL's go/disabled-certificate-check passes over functions named for
// this, so renaming or inlining it brings the alert back.
func selfSignedTransport() *http.Transport {
	return &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
}

// Vars reads the named variables in one request.
func (c *Client) Vars(ctx context.Context, names ...string) (map[string]string, error) {
	return c.query(ctx, nameQuery+strings.Join(names, ","))
}

// Match reads every variable whose name contains s.
func (c *Client) Match(ctx context.Context, s string) (map[string]string, error) {
	return c.query(ctx, matchQuery+s)
}

// EnableLive switches on the live data websocket. A PVS5 has no such switch
// and answers ErrBadRequest.
func (c *Client) EnableLive(ctx context.Context) error {
	_, err := c.query(ctx, setQuery+liveVar+"=1")
	return err
}

func (c *Client) query(ctx context.Context, q string) (map[string]string, error) {
	if err := c.login(ctx); err != nil {
		return nil, err
	}
	vars, err := c.post(ctx, q)
	if !errors.Is(err, errExpired) {
		return vars, err
	}
	c.mu.Lock()
	c.loggedIn = false
	c.mu.Unlock()
	if err := c.login(ctx); err != nil {
		return nil, err
	}
	return c.post(ctx, q)
}

// login logs in unless already logged in. Without a session the PVS answers
// 400, not 401, so the first request cannot be what triggers it.
func (c *Client) login(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loggedIn {
		return nil
	}
	if c.password == "" {
		// the serial number reads without a session
		vars, err := c.post(ctx, nameQuery+serialVar)
		if err != nil {
			return fmt.Errorf("reading the serial number: %w", err)
		}
		serial := vars[serialVar]
		if len(serial) < passwordLen {
			return fmt.Errorf("serial number %q is too short for the default password", serial)
		}
		c.password = serial[len(serial)-passwordLen:]
	}
	if err := c.authenticate(ctx); err != nil {
		return err
	}
	c.loggedIn = true
	return nil
}

func (c *Client) authenticate(ctx context.Context) (err error) {
	defer func() { c.count("login", err != nil) }()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+loginPath, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(loginUser, c.password)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("logging in: PVS answered %s", resp.Status)
	}
	return nil
}

func (c *Client) post(ctx context.Context, q string) (vars map[string]string, err error) {
	kind, _, _ := strings.Cut(q, "=")
	defer func() { c.count(kind, err != nil && !errors.Is(err, ErrBadRequest) && !errors.Is(err, errExpired)) }()
	// raw, not URL-encoded: the PVS takes paths and set=name=value as they are
	body := strings.NewReader(q + objFormat)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+varsPath, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		if err := json.NewDecoder(resp.Body).Decode(&vars); err != nil {
			return nil, fmt.Errorf("decoding the PVS's answer: %w", err)
		}
		return vars, nil
	case http.StatusUnauthorized:
		return nil, errExpired
	case http.StatusBadRequest:
		return nil, ErrBadRequest
	default:
		return nil, fmt.Errorf("PVS answered %s", resp.Status)
	}
}
