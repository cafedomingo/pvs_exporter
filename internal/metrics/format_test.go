package metrics

import (
	"testing"
)

func TestFormatGroupsByName(t *testing.T) {
	samples := []Sample{
		{Name: "pvs_meter_power_watts", Labels: []Label{{Name: "serial", Value: "M0"}}, Value: 3200},
		{Name: "pvs_up", Value: 1},
		{Name: "pvs_meter_power_watts", Labels: []Label{{Name: "serial", Value: "M1"}}, Value: 0.5},
	}

	want := `# TYPE pvs_meter_power_watts gauge
pvs_meter_power_watts{serial="M0"} 3200
pvs_meter_power_watts{serial="M1"} 0.5
# TYPE pvs_up gauge
pvs_up 1
`
	if got := string(Format(samples)); got != want {
		t.Errorf("Format =\n%s\nwant\n%s", got, want)
	}
}

func TestFormatTypesCounters(t *testing.T) {
	samples := []Sample{{Name: "pvs_requests_total", Value: 3, Counter: true}}

	want := "# TYPE pvs_requests_total counter\npvs_requests_total 3\n"
	if got := string(Format(samples)); got != want {
		t.Errorf("Format = %q, want %q", got, want)
	}
}

func TestFormatEscapesLabelValues(t *testing.T) {
	samples := []Sample{{Name: "x", Labels: []Label{{Name: "v", Value: "a\\b\"c\nd"}}, Value: 1}}

	want := "# TYPE x gauge\nx{v=\"a\\\\b\\\"c\\nd\"} 1\n"
	if got := string(Format(samples)); got != want {
		t.Errorf("Format = %q, want %q", got, want)
	}
}

func TestAppendPush(t *testing.T) {
	samples := []Sample{
		{Name: "pvs_live_power_watts", Labels: []Label{{Name: "flow", Value: "pv"}}, Value: 3200},
		{Name: "pvs_live_timestamp_seconds", Value: 1791944553},
	}
	extra := []Label{{Name: "job", Value: "pvs"}, {Name: "instance", Value: "pvs-exporter:9711"}}

	want := `pvs_live_power_watts{flow="pv",job="pvs",instance="pvs-exporter:9711"} 3200 1791944553000
pvs_live_timestamp_seconds{job="pvs",instance="pvs-exporter:9711"} 1.791944553e+09 1791944553000
`
	if got := string(AppendPush(nil, samples, extra, 1791944553000)); got != want {
		t.Errorf("AppendPush =\n%s\nwant\n%s", got, want)
	}
}
