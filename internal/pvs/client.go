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
	"time"
)

// ErrBadRequest is the PVS's answer to an unknown variable name, and to a
// match that finds nothing.
var ErrBadRequest = errors.New("PVS answered 400 Bad Request")

var errExpired = errors.New("PVS session expired")

// Client reads varserver variables. It logs in before its first request and
// again when the session expires.
type Client struct {
	base string
	http *http.Client

	mu       sync.Mutex
	password string
	loggedIn bool
}

// NewClient returns a client for the PVS at host. An empty password is
// derived from the serial number, as the PVS does by default.
func NewClient(host, password string, timeout time.Duration) *Client {
	jar, _ := cookiejar.New(nil) // fails only for a bad public suffix list
	return &Client{
		base:     "https://" + host,
		password: password,
		http: &http.Client{
			Jar:     jar,
			Timeout: timeout,
			// the PVS has a self-signed certificate
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		},
	}
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
	c.loggedIn = true
	return nil
}

func (c *Client) post(ctx context.Context, q string) (map[string]string, error) {
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
		var vars map[string]string
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
