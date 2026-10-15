package pvs_test

import (
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cafedomingo/pvs_exporter/internal/pvs"
	"github.com/cafedomingo/pvs_exporter/internal/pvstest"
)

const uptime = "/sys/info/uptime"

func fake(t *testing.T) *pvstest.PVS {
	return pvstest.New(t, map[string]string{
		uptime:                    "3484.04",
		"/sys/info/sys_type":      "Storage",
		"/sys/telemetryws/enable": "0",
		"/sys/devices/meter/0/sn": "FAKE-METER-0",
	})
}

func TestVarsLogsInWithTheSerialDerivedPassword(t *testing.T) {
	p := fake(t)
	c := pvs.NewClient(p.Host(), "", time.Second)

	vars, err := c.Vars(t.Context(), uptime, "/sys/info/sys_type")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{uptime: "3484.04", "/sys/info/sys_type": "Storage"}
	if !maps.Equal(vars, want) {
		t.Errorf("Vars = %v, want %v", vars, want)
	}
	queries := []string{
		"name=/sys/info/serialnum&fmt=obj",
		"name=/sys/info/uptime,/sys/info/sys_type&fmt=obj",
	}
	if got := p.Queries(); !slices.Equal(got, queries) {
		t.Errorf("queries = %q, want %q", got, queries)
	}
}

func TestVarsLogsInAgainWhenTheSessionExpires(t *testing.T) {
	p := fake(t)
	c := pvs.NewClient(p.Host(), "", time.Second)
	if _, err := c.Vars(t.Context(), uptime); err != nil {
		t.Fatal(err)
	}
	p.Expire()

	if _, err := c.Vars(t.Context(), uptime); err != nil {
		t.Fatalf("after expiry: %v", err)
	}
}

func TestRequestsAreCountedByKind(t *testing.T) {
	p := fake(t)
	c := pvs.NewClient(p.Host(), "", time.Second)
	_, _ = c.Vars(t.Context(), uptime)      // serial read, login, read
	_, _ = c.Vars(t.Context(), "/sys/nope") // 400
	_, _ = c.Match(t.Context(), "/sys/devices/")
	_ = c.EnableLive(t.Context())
	p.Expire()
	_, _ = c.Vars(t.Context(), uptime) // 401, login, read

	want := []pvs.RequestCount{
		{Kind: "login", Total: 2},
		{Kind: "name", Total: 5},
		{Kind: "match", Total: 1},
		{Kind: "set", Total: 1},
	}
	if got := c.Requests(); !slices.Equal(got, want) {
		t.Errorf("Requests = %+v, want %+v", got, want)
	}
}

func TestVarsFailsWithTheWrongPassword(t *testing.T) {
	p := fake(t)
	c := pvs.NewClient(p.Host(), "wrong", time.Second)

	if _, err := c.Vars(t.Context(), uptime); err == nil {
		t.Fatal("Vars succeeded with the wrong password")
	}
	if got, want := c.Requests()[0], (pvs.RequestCount{Kind: "login", Total: 1, Failures: 1}); got != want {
		t.Errorf("logins = %+v, want %+v", got, want)
	}
}

func TestUnknownNameIsBadRequest(t *testing.T) {
	p := fake(t)
	c := pvs.NewClient(p.Host(), "", time.Second)

	_, err := c.Vars(t.Context(), uptime, "/sys/nope")
	if !errors.Is(err, pvs.ErrBadRequest) {
		t.Fatalf("err = %v, want ErrBadRequest", err)
	}
}

func TestMatch(t *testing.T) {
	p := fake(t)
	c := pvs.NewClient(p.Host(), "", time.Second)

	vars, err := c.Match(t.Context(), "/sys/devices/")
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"/sys/devices/meter/0/sn": "FAKE-METER-0"}; !maps.Equal(vars, want) {
		t.Errorf("Match = %v, want %v", vars, want)
	}
	if _, err := c.Match(t.Context(), "/sys/devices/inverter/"); !errors.Is(err, pvs.ErrBadRequest) {
		t.Errorf("empty match: err = %v, want ErrBadRequest", err)
	}
}

func TestEnableLive(t *testing.T) {
	p := fake(t)
	c := pvs.NewClient(p.Host(), "", time.Second)

	if err := c.EnableLive(t.Context()); err != nil {
		t.Fatal(err)
	}
	vars, err := c.Vars(t.Context(), "/sys/telemetryws/enable")
	if err != nil {
		t.Fatal(err)
	}
	if got := vars["/sys/telemetryws/enable"]; got != "1" {
		t.Errorf("enable = %q, want 1", got)
	}
}

func TestVarsTimesOutWhenThePVSNeverAnswers(t *testing.T) {
	done := make(chan struct{})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-done }))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(done) })
	c := pvs.NewClient(strings.TrimPrefix(srv.URL, "https://"), "", 100*time.Millisecond)

	start := time.Now()
	_, err := c.Vars(t.Context(), uptime)

	if err == nil || time.Since(start) > time.Second {
		t.Errorf("Vars returned %v after %v, want a timeout after about 100ms", err, time.Since(start))
	}
	if got, want := c.Requests()[1], (pvs.RequestCount{Kind: "name", Total: 1, Failures: 1}); got != want {
		t.Errorf("name reads = %+v, want %+v", got, want)
	}
}
