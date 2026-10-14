package metrics

import (
	"encoding/json"
	"math"
	"os"
	"slices"
	"testing"

	"github.com/cafedomingo/pvs_exporter/internal/pvs"
)

// fixture is a match=/sys/devices/ answer from the homelab PVS6, with serial
// numbers replaced. The inverter, absent at night, is from doc/LocalAPI.md
// plus the AC params a daytime read carries.
func fixture(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile("testdata/devices.json")
	if err != nil {
		t.Fatal(err)
	}
	var vars map[string]string
	if err := json.Unmarshal(data, &vars); err != nil {
		t.Fatal(err)
	}
	return vars
}

// value finds the sample with name whose labels include want.
func value(t *testing.T, samples []Sample, name string, want ...Label) float64 {
	t.Helper()
	for _, s := range samples {
		if s.Name == name && hasLabels(s, want) {
			return s.Value
		}
	}
	t.Fatalf("no %s%v in %v", name, want, samples)
	return 0
}

func hasLabels(s Sample, want []Label) bool {
	for _, w := range want {
		if !slices.Contains(s.Labels, w) {
			return false
		}
	}
	return true
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestDevices(t *testing.T) {
	samples := Devices(pvs.Devices(fixture(t)))

	meter := Label{"serial", "FAKE-METER-1"}
	tests := []struct {
		name   string
		labels []Label
		want   float64
	}{
		{"pvs_meter_power_watts", []Label{meter, {"model", "PVS6M0400c"}}, 710.282},
		{"pvs_meter_leg_power_watts", []Label{meter, {"leg", "2"}}, 260.967},
		{"pvs_meter_reactive_power_var", []Label{meter}, -735.327},
		{"pvs_meter_last_report_timestamp_seconds", []Label{meter}, 1791966300},
		{"pvs_ess_soc_ratio", []Label{{"serial", "FAKE-ESS-0"}}, 0.32},
		{"pvs_ess_charge_limit_watts", []Label{{"serial", "FAKE-ESS-0"}}, 10762},
		{"pvs_ess_info", []Label{{"op_mode", "TARIFF_OPTIMIZER"}}, 1},
		{"pvs_inverter_dc_power_watts", []Label{{"serial", "FAKE-INVERTER-0"}}, 301},
		{"pvs_transfer_switch_info", []Label{{"mid_state", "MID_STATE_CLOSED"}, {"pvd1_state", "PVD_STATE_CLOSED"}}, 1},
		{"pvs_transfer_switch_grid_voltage_volts", []Label{{"leg", "1"}}, 122.522003},
	}
	for _, tt := range tests {
		if got := value(t, samples, tt.name, tt.labels...); !near(got, tt.want) {
			t.Errorf("%s%v = %v, want %v", tt.name, tt.labels, got, tt.want)
		}
	}
}

func TestDevicesOmitsMissingReadings(t *testing.T) {
	devices := map[string][]pvs.Device{"meter": {{"sn": "M0", "p3phsumKw": "N/A", "msmtEps": ""}}}

	if got := Devices(devices); len(got) != 0 {
		t.Errorf("Devices = %v, want none", got)
	}
}

// Every param the PVS reported is exported, so nothing it measures is lost.
func TestDeviceFieldsCoverTheFixture(t *testing.T) {
	ignored := []string{"sn", "prodMdlNm", "vldFldMsk", "vldFld2Msk"}
	for kind, devices := range pvs.Devices(fixture(t)) {
		for param := range devices[0] {
			mapped := slices.ContainsFunc(deviceFields[kind], func(f field) bool { return f.key == param }) ||
				slices.ContainsFunc(deviceInfo[kind].fields, func(f infoField) bool { return f.key == param })
			if !mapped && !slices.Contains(ignored, param) {
				t.Errorf("%s param %s is not exported", kind, param)
			}
		}
	}
}
