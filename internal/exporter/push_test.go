package exporter

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cafedomingo/pvs_exporter/internal/metrics"
	"github.com/cafedomingo/pvs_exporter/internal/pvs"
)

// receiver records pushes and answers with status.
type receiver struct {
	mu     sync.Mutex
	status int
	bodies []string
	auth   string
}

func newReceiver(t *testing.T) (*receiver, string) {
	r := &receiver{status: http.StatusNoContent}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		defer r.mu.Unlock()
		r.bodies = append(r.bodies, string(body))
		user, password, _ := req.BasicAuth()
		r.auth = user + ":" + password
		w.WriteHeader(r.status)
	}))
	t.Cleanup(srv.Close)
	return r, srv.URL
}

func frame(time int64, pv float64) pvs.Frame {
	return pvs.Frame{Time: time, Params: map[string]float64{"pv_p": pv}}
}

func TestPushSendsQueuedFramesInOrder(t *testing.T) {
	r, url := newReceiver(t)
	p := newPusher(Config{
		PushURL: url, PushUser: "u", PushPassword: "p",
		PushLabels: []metrics.Label{{Name: "job", Value: "pvs"}},
	})
	p.add(frame(100, 3.2))
	p.add(frame(101, 3.3))

	p.push(t.Context())

	want := `pvs_live_power_watts{flow="pv",job="pvs"} 3200 100000
pvs_live_timestamp_seconds{job="pvs"} 100 100000
pvs_live_power_watts{flow="pv",job="pvs"} 3300 101000
pvs_live_timestamp_seconds{job="pvs"} 101 101000
`
	if len(r.bodies) != 1 || r.bodies[0] != want {
		t.Errorf("pushed %q, want %q", r.bodies, want)
	}
	if r.auth != "u:p" {
		t.Errorf("auth = %q, want u:p", r.auth)
	}
	if len(p.queue) != 0 || p.queued.Load() != 0 {
		t.Errorf("%d frames left after a good push, %d reported", len(p.queue), p.queued.Load())
	}
}

func TestPushKeepsFramesUntilAPushSucceeds(t *testing.T) {
	r, url := newReceiver(t)
	p := newPusher(Config{PushURL: url})
	r.status = http.StatusServiceUnavailable
	p.add(frame(100, 3.2))
	p.push(t.Context())
	p.add(frame(101, 3.3))
	r.status = http.StatusNoContent

	p.push(t.Context())

	last := r.bodies[len(r.bodies)-1]
	if !strings.Contains(last, " 100000\n") || !strings.Contains(last, " 101000\n") ||
		strings.Index(last, " 100000\n") > strings.Index(last, " 101000\n") {
		t.Errorf("retry pushed %q, want both frames in order", last)
	}
	if len(p.queue) != 0 {
		t.Errorf("%d frames left after a good push", len(p.queue))
	}
}

func TestQueueDropsTheOldestPastTheCap(t *testing.T) {
	p := newPusher(Config{PushURL: "http://unused"})
	for i := range queueMax + 1 {
		p.add(frame(int64(i+1), 0))
	}

	if len(p.queue) != queueMax || p.queue[0].Time != 2 {
		t.Errorf("queue has %d frames from time %d, want %d from 2", len(p.queue), p.queue[0].Time, queueMax)
	}
	if p.queued.Load() != int64(queueMax) || p.dropped.Load() != 1 {
		t.Errorf("queued = %d, dropped = %d; want %d, 1", p.queued.Load(), p.dropped.Load(), queueMax)
	}
}

func TestPushSkipsARepeatedTime(t *testing.T) {
	p := newPusher(Config{PushURL: "http://unused"})

	p.add(frame(100, 3.2))
	p.add(frame(100, 9.9))

	if len(p.queue) != 1 || p.queue[0].Params["pv_p"] != 3.2 {
		t.Errorf("queue = %v, want only the first frame at time 100", p.queue)
	}
}

func TestPushKeepsFramesWhenTheReceiverIsDown(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close() // connections are refused
	p := newPusher(Config{PushURL: srv.URL})
	p.add(frame(100, 3.2))

	p.push(t.Context())

	if len(p.queue) != 1 {
		t.Errorf("%d frames queued after a refused push, want 1", len(p.queue))
	}
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRunPushesOnEachTickUntilAPushSucceeds(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		pushed := make(chan string, 4)
		status := http.StatusServiceUnavailable
		p := newPusher(Config{PushURL: "http://receiver/"})
		p.http.Transport = roundTripper(func(r *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(r.Body)
			pushed <- string(body)
			return &http.Response{StatusCode: status, Body: http.NoBody, Request: r}, nil
		})
		ctx, cancel := context.WithCancel(t.Context())
		frames := make(chan pvs.Frame)
		done := make(chan struct{})
		go func() {
			p.run(ctx, frames)
			close(done)
		}()

		frames <- frame(100, 3.2)
		synctest.Wait()
		if len(pushed) != 0 {
			t.Fatal("pushed before the first tick")
		}
		time.Sleep(pushEvery + time.Second) // the first tick: the receiver is down
		synctest.Wait()
		status = http.StatusNoContent // ordered before the push by the send below
		frames <- frame(101, 3.3)
		time.Sleep(pushEvery) // the second tick
		synctest.Wait()
		cancel()
		<-done
		close(pushed)

		var bodies []string
		for b := range pushed {
			bodies = append(bodies, b)
		}
		if len(bodies) != 2 || !strings.Contains(bodies[1], " 100000\n") || !strings.Contains(bodies[1], " 101000\n") {
			t.Errorf("pushed %q, want a failed push of 100 then one of 100 and 101", bodies)
		}
	})
}
