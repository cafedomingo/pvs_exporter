package metrics

import (
	"testing"

	"github.com/cafedomingo/pvs_exporter/v2/internal/pvs"
)

func TestLive(t *testing.T) {
	f := pvs.Frame{Time: 1791944553, Params: map[string]float64{
		"soc": 0.68, "site_load_p": 1.248, "net_p": 0.055, "pv_p": 0, "ess_p": 1.193,
		"site_load_en": 46691.564, "net_en": 17245.71, "pv_en": 31374.93, "ess_en": -1929.076,
	}}
	samples := Live(f)

	if len(samples) != 10 {
		t.Errorf("got %d samples, want 10", len(samples))
	}
	tests := []struct {
		name string
		flow string
		want float64
	}{
		{"pvs_live_power_watts", "load", 1248},
		{"pvs_live_power_watts", "battery", 1193},
		{"pvs_live_power_watts", "pv", 0},
		{"pvs_live_energy_kwh", "net", 17245.71},
		{"pvs_live_energy_kwh", "battery", -1929.076},
	}
	for _, tt := range tests {
		if got := value(t, samples, tt.name, Label{"flow", tt.flow}); !near(got, tt.want) {
			t.Errorf("%s{flow=%s} = %v, want %v", tt.name, tt.flow, got, tt.want)
		}
	}
	if got := value(t, samples, "pvs_live_soc_ratio"); got != 0.68 {
		t.Errorf("soc = %v, want 0.68", got)
	}
	if got := value(t, samples, "pvs_live_timestamp_seconds"); got != 1791944553 {
		t.Errorf("timestamp = %v, want 1791944553", got)
	}
}
