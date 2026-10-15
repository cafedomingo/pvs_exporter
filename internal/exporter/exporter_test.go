package exporter

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cafedomingo/pvs_exporter/v2/internal/pvstest"
)

const liveFrame = `{"notification": "power", "params": {"time": 1791944553, "pv_p": 3.2, "soc": 0.68}}`

// start runs the exporter against a fake PVS until the test ends and returns
// its /metrics URL.
func start(t *testing.T, cfg Config) string {
	t.Helper()
	fake := pvstest.New(t, gateway)
	fake.Set(meter)
	cfg.PVSHost = fake.Host()
	cfg.LiveURL = pvstest.Live(t, 0, liveFrame)
	return run(t, cfg)
}

// run runs the exporter until the test ends and returns its /metrics URL.
func run(t *testing.T, cfg Config) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- Run(ctx, cfg, ln) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Run: %v", err)
		}
	})
	return "http://" + ln.Addr().String() + "/metrics"
}

// scrape fetches url until its body has every one of want, or fails after 5s.
func scrape(t *testing.T, url string, want ...string) string {
	t.Helper()
	var body string
	for range 250 {
		if resp, err := http.Get(url); err == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			body = string(b)
			if hasAll(body, want) {
				return body
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("/metrics never had %q:\n%s", want, body)
	return ""
}

func hasAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func TestRunServesPolledAndLiveMetrics(t *testing.T) {
	url := start(t, Config{})

	scrape(t, url,
		"pvs_up 1\n",
		"pvs_live_connected 1\n",
		`pvs_meter_power_watts{serial="FAKE-METER-0",model=""} 710`,
		`pvs_gateway_cpu_usage_ratio 0.11`,
		`pvs_live_power_watts{flow="pv"} 3200`,
		`pvs_live_soc_ratio 0.68`,
	)
}

func TestRunServesItsOwnStats(t *testing.T) {
	url := start(t, Config{})

	body := scrape(t, url,
		"pvs_live_frames_total 1\n",
		`pvs_requests_total{request="match"} 1`,
	)
	for _, want := range []string{
		`pvs_exporter_build_info{version="(devel)",goversion="` + runtime.Version() + `"} 1`,
		`pvs_requests_total{request="login"} 1`,
		`pvs_request_failures_total{request="name"} 0`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "pvs_push_") {
		t.Errorf("push stats without push:\n%s", body)
	}
}

func TestRunWithPushKeepsLiveValuesOffMetrics(t *testing.T) {
	_, pushURL := newReceiver(t)
	url := start(t, Config{PushURL: pushURL})

	body := scrape(t, url, "pvs_up 1\n", "pvs_live_connected 1\n", "pvs_push_queued_frames ", "pvs_push_dropped_frames_total 0\n")
	if strings.Contains(body, "pvs_live_power_watts") {
		t.Errorf("live values on /metrics while pushing:\n%s", body)
	}
}

func TestRunServesStatusWhileThePVSIsDown(t *testing.T) {
	fake := pvstest.New(t, gateway)
	fake.Close()
	url := run(t, Config{PVSHost: fake.Host(), LiveURL: "ws://127.0.0.1:1"})

	scrape(t, url, "pvs_up 0\n", "pvs_live_connected 0\n")
}

func TestRunStopsPromptlyWhileThePVSHangs(t *testing.T) {
	done, reached := make(chan struct{}), make(chan struct{})
	var once sync.Once
	hung := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		once.Do(func() { close(reached) })
		<-done
	}))
	t.Cleanup(hung.Close)
	t.Cleanup(func() { close(done) })
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	stopped := make(chan error)
	go func() {
		stopped <- Run(ctx, Config{PVSHost: strings.TrimPrefix(hung.URL, "https://"), LiveURL: "ws://127.0.0.1:1"}, ln)
	}()
	<-reached // a request is now waiting on the PVS

	cancel()

	select {
	case err := <-stopped:
		if err != nil {
			t.Errorf("Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run still running 2s after its context ended")
	}
}
