package exporter

import (
	"log/slog"
	"reflect"
	"testing"

	"github.com/cafedomingo/pvs_exporter/v2/internal/metrics"
)

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestConfigFromEnv(t *testing.T) {
	c, err := ConfigFromEnv(env(map[string]string{
		"PVS_HOST":    "192.0.2.10",
		"PUSH_URL":    "http://vm:8428/api/v1/import/prometheus",
		"PUSH_LABELS": "job=pvs,instance=pvs-exporter:9711",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Listen:     ":9711",
		PVSHost:    "192.0.2.10",
		LiveURL:    "ws://192.0.2.10:9002",
		PushURL:    "http://vm:8428/api/v1/import/prometheus",
		PushLabels: []metrics.Label{{Name: "job", Value: "pvs"}, {Name: "instance", Value: "pvs-exporter:9711"}},
	}
	if !reflect.DeepEqual(c, want) {
		t.Errorf("config = %+v, want %+v", c, want)
	}
}

func TestConfigFromEnvReadsTheLogLevel(t *testing.T) {
	c, err := ConfigFromEnv(env(map[string]string{"PVS_HOST": "pvs", "LOG_LEVEL": "debug"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.LogLevel != slog.LevelDebug {
		t.Errorf("log level = %v, want DEBUG", c.LogLevel)
	}
}

func TestConfigFromEnvRejects(t *testing.T) {
	for name, vars := range map[string]map[string]string{
		"no host":   {},
		"bad label": {"PVS_HOST": "pvs", "PUSH_LABELS": "job"},
		"no name":   {"PVS_HOST": "pvs", "PUSH_LABELS": "=pvs"},
		"bad level": {"PVS_HOST": "pvs", "LOG_LEVEL": "loud"},
	} {
		if _, err := ConfigFromEnv(env(vars)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
