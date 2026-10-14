package exporter

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestOutageWarnsOncePerOutage(t *testing.T) {
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
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
