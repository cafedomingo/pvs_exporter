package exporter

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

// captureLogs sends the default logger, at debug, to the returned buffer until
// the test ends.
func captureLogs(t *testing.T) *bytes.Buffer {
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &logs
}

func TestOutageWarnsOncePerOutage(t *testing.T) {
	logs := captureLogs(t)
	o := outage{what: "polling"}

	o.ok()
	o.fail(errors.New("timeout"))
	o.fail(errors.New("timeout"))
	o.ok()
	o.ok()

	var levels []string
	for line := range strings.Lines(logs.String()) {
		for field := range strings.FieldsSeq(line) {
			if level, ok := strings.CutPrefix(field, "level="); ok {
				levels = append(levels, level)
			}
		}
	}
	if got, want := strings.Join(levels, " "), "WARN DEBUG INFO"; got != want {
		t.Errorf("levels = %s, want %s\n%s", got, want, logs.String())
	}
}

func TestOutageRecoveryLogsHowLongItLasted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		logs := captureLogs(t)
		o := outage{what: "polling"}
		o.fail(errors.New("timeout"))
		time.Sleep(time.Minute)
		o.fail(errors.New("timeout"))
		time.Sleep(30 * time.Second)
		o.ok()
		if !strings.Contains(logs.String(), `msg="polling recovered" after=1m30s`) {
			t.Errorf("no recovery with its duration in\n%s", logs.String())
		}
	})
}
