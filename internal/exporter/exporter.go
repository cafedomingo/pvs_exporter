package exporter

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cafedomingo/pvs_exporter/internal/metrics"
	"github.com/cafedomingo/pvs_exporter/internal/pvs"
)

type snapshot struct {
	up       bool
	duration time.Duration
	samples  []metrics.Sample
}

type exporter struct {
	cfg        Config
	client     *pvs.Client
	build      metrics.Sample
	snap       atomic.Pointer[snapshot]
	connected  atomic.Bool
	liveFrames atomic.Uint64
	latest     atomic.Pointer[pvs.Frame] // without push: served on /metrics
	frames     chan pvs.Frame            // with push
	pusher     *pusher                   // with push
}

// Run serves /metrics on ln, polls the PVS, and streams its live data until ctx
// ends.
func Run(ctx context.Context, cfg Config, ln net.Listener) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	info, _ := debug.ReadBuildInfo()
	e := &exporter{
		cfg:    cfg,
		client: pvs.NewClient(cfg.PVSHost, cfg.PVSPassword, requestTimeout),
		build: metrics.Sample{Name: "pvs_exporter_build_info", Value: 1, Labels: []metrics.Label{
			{Name: "version", Value: info.Main.Version}, {Name: "goversion", Value: runtime.Version()},
		}},
	}
	e.snap.Store(&snapshot{})
	var wg sync.WaitGroup
	if cfg.PushURL != "" {
		e.frames = make(chan pvs.Frame, frameBuffer)
		e.pusher = newPusher(cfg)
		wg.Go(func() { e.pusher.run(ctx, e.frames) })
	}
	wg.Go(func() { e.pollLoop(ctx) })
	wg.Go(func() { e.liveLoop(ctx) })

	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", e.serveMetrics)
	srv := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	err := srv.Serve(ln)
	cancel()
	wg.Wait()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (e *exporter) pollLoop(ctx context.Context) {
	p := newPoller(e.client)
	o := outage{what: "polling the PVS"}
	tick := time.NewTicker(coreEvery)
	defer tick.Stop()
	for {
		start := time.Now()
		pollCtx, cancel := context.WithTimeout(ctx, pollTimeout)
		samples, err := p.poll(pollCtx, start)
		cancel()
		if ctx.Err() != nil {
			return
		}
		e.snap.Store(&snapshot{up: err == nil, duration: time.Since(start), samples: samples})
		if err != nil {
			o.fail(err)
		} else {
			o.ok()
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func (e *exporter) liveLoop(ctx context.Context) {
	o := outage{what: "streaming live data"}
	for {
		if err := e.client.EnableLive(ctx); err != nil {
			slog.Debug("enabling the live data websocket failed", "err", err)
		}
		err := pvs.Stream(ctx, e.cfg.LiveURL, liveStall, func(f pvs.Frame) {
			e.liveFrames.Add(1)
			e.connected.Store(true)
			o.ok()
			e.frame(ctx, f)
		})
		e.connected.Store(false)
		e.latest.Store(nil)
		if ctx.Err() != nil {
			return
		}
		o.fail(err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(liveRetry):
		}
	}
}

func (e *exporter) frame(ctx context.Context, f pvs.Frame) {
	if e.frames == nil {
		e.latest.Store(&f)
		return
	}
	select {
	case e.frames <- f:
	case <-ctx.Done():
	}
}

func (e *exporter) serveMetrics(w http.ResponseWriter, _ *http.Request) {
	s := e.snap.Load()
	samples := []metrics.Sample{
		e.build,
		{Name: "pvs_up", Value: gauge(s.up)},
		{Name: "pvs_poll_duration_seconds", Value: s.duration.Seconds()},
		{Name: "pvs_live_connected", Value: gauge(e.connected.Load())},
		{Name: "pvs_live_frames_total", Value: float64(e.liveFrames.Load()), Counter: true},
	}
	for _, r := range e.client.Requests() {
		kind := []metrics.Label{{Name: "request", Value: r.Kind}}
		samples = append(samples,
			metrics.Sample{Name: "pvs_requests_total", Labels: kind, Value: float64(r.Total), Counter: true},
			metrics.Sample{Name: "pvs_request_failures_total", Labels: kind, Value: float64(r.Failures), Counter: true})
	}
	if p := e.pusher; p != nil {
		samples = append(samples,
			metrics.Sample{Name: "pvs_push_queued_frames", Value: float64(p.queued.Load())},
			metrics.Sample{Name: "pvs_push_dropped_frames_total", Value: float64(p.dropped.Load()), Counter: true})
	}
	samples = append(samples, s.samples...)
	if f := e.latest.Load(); f != nil {
		samples = append(samples, metrics.Live(*f)...)
	}
	w.Header().Set("Content-Type", metrics.ContentType)
	_, _ = w.Write(metrics.Format(samples)) // the scraper went away
}

func gauge(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
