package exporter

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cafedomingo/pvs_exporter/internal/metrics"
	"github.com/cafedomingo/pvs_exporter/internal/pvs"
)

// pusher sends live frames to PUSH_URL, keeping them in order until a push
// succeeds.
type pusher struct {
	cfg    Config
	http   *http.Client
	queue  []pvs.Frame
	last   int64 // time of the last frame added
	outage outage
}

func newPusher(c Config) *pusher {
	return &pusher{cfg: c, http: &http.Client{Timeout: pushEvery}, outage: outage{what: "pushing live data"}}
}

func (p *pusher) run(ctx context.Context, frames <-chan pvs.Frame) {
	tick := time.NewTicker(pushEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case f := <-frames:
			p.add(f)
		case <-tick.C:
			p.push(ctx)
		}
	}
}

func (p *pusher) add(f pvs.Frame) {
	// a repeated time, as after a reconnect, would stamp new values with an
	// old timestamp
	if f.Time == p.last {
		return
	}
	p.last = f.Time
	if len(p.queue) == queueMax {
		p.queue = p.queue[1:]
	}
	p.queue = append(p.queue, f)
}

func (p *pusher) push(ctx context.Context) {
	if len(p.queue) == 0 {
		return
	}
	var body []byte
	for _, f := range p.queue {
		body = metrics.AppendPush(body, metrics.Live(f), p.cfg.PushLabels, time.Unix(f.Time, 0).UnixMilli())
	}
	if err := p.send(ctx, body); err != nil {
		p.outage.fail(err)
		return
	}
	p.outage.ok()
	p.queue = p.queue[:0]
}

func (p *pusher) send(ctx context.Context, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.PushURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", metrics.ContentType)
	if p.cfg.PushUser != "" {
		req.SetBasicAuth(p.cfg.PushUser, p.cfg.PushPassword)
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("push answered %s", resp.Status)
	}
	return nil
}
