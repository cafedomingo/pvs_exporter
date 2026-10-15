package exporter

import (
	"context"
	"log/slog"
	"time"
)

// outage logs a failing activity at warn the first time, at debug while it
// keeps failing, and at info once it recovers. One goroutine owns each.
type outage struct {
	what    string
	failing bool
	since   time.Time
}

func (o *outage) fail(err error) {
	level := slog.LevelWarn
	if o.failing {
		level = slog.LevelDebug
	} else {
		o.since = time.Now()
	}
	slog.Log(context.Background(), level, o.what+" failed", "err", err)
	o.failing = true
}

func (o *outage) ok() {
	if o.failing {
		slog.Info(o.what+" recovered", "after", time.Since(o.since).Round(time.Second))
		o.failing = false
	}
}
