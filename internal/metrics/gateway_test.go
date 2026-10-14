package metrics

import (
	"maps"
	"slices"
	"testing"
)

// core and extras are answers from the homelab PVS6 (Storage). The tests spell
// out the variable names, so a wrong name in package pvs fails them.
var (
	core = map[string]string{
		"/sys/info/cpu_usage":               "0",
		"/sys/info/ram_usage":               "7",
		"/sys/info/flash_usage":             "73",
		"/sys/info/uptime":                  "4201.33",
		"/sys/info/model":                   "PVS6",
		"/sys/info/hwrev":                   "6.02",
		"/sys/info/sw_rev":                  "2025.10.20.61846",
		"/sys/info/sys_type":                "Storage",
		"/sys/livedata/backupTimeRemaining": "167",
		"/sys/livedata/midstate":            "2",
	}
	extras = map[string]string{
		"/sys/info/active_interface":                     "wan0",
		"/sys/info/boardtype":                            "A0K LIB 1.0 PSoC5",
		"/sys/info/fwrev":                                "",
		"/sys/pvs/flashwear_type_b":                      "0x01",
		"/sys/pvs/usb_erase_count":                       "0",
		"/sys/toggle_cell/broadband_connected":           "1",
		"/sys/toggle_cell/cell_connected":                "0",
		"/sys/toggle_cell/low_data_mode":                 "0",
		"/ess/config/dcm/control_param/min_customer_soc": "0.20",
		"/ess/config/dcm/mode_param/control_mode":        "SELF_CONSUMPTION",
		"/net/sta0/state":                                "ready",
		"/net/wan0/state":                                "online",
		"/net/wan1/state":                                "N/A",
		"/net/wwan0/state":                               "N/A",
	}
)

func TestGateway(t *testing.T) {
	vars := maps.Clone(core)
	maps.Copy(vars, extras)
	samples := Gateway(vars)

	for name, want := range map[string]float64{
		"pvs_gateway_cpu_usage_ratio":            0,
		"pvs_gateway_flash_usage_ratio":          0.73,
		"pvs_gateway_ram_usage_ratio":            0.07,
		"pvs_gateway_flash_wear_ratio":           0.1,
		"pvs_gateway_uptime_seconds":             4201.33,
		"pvs_gateway_usb_erase_count":            0,
		"pvs_gateway_broadband_connected":        1,
		"pvs_gateway_cell_connected":             0,
		"pvs_gateway_low_data_mode":              0,
		"pvs_gateway_ess_reserve_soc_ratio":      0.2,
		"pvs_live_backup_time_remaining_seconds": 10020,
		"pvs_live_mid_state":                     2,
	} {
		if got := value(t, samples, name); !near(got, want) {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	info := []Label{
		{"active_interface", "wan0"}, {"board_type", "A0K LIB 1.0 PSoC5"},
		{"ess_control_mode", "SELF_CONSUMPTION"}, {"firmware_revision", ""},
		{"pvs_type", "PVS6"}, {"software_version", "2025.10.20.61846"}, {"sys_type", "Storage"},
		{"hardware_version", "PVS6 6.02"},
	}
	value(t, samples, "pvs_gateway_info", info...)
	for iface, state := range map[string]string{"sta0": "ready", "wan0": "online", "wan1": "N/A", "wwan0": "N/A"} {
		value(t, samples, "pvs_gateway_interface_info", Label{"interface", iface}, Label{"state", state})
	}
}

func TestExtraVars(t *testing.T) {
	storage := ExtraVars(core)
	for _, v := range []string{"/sys/pvs/flashwear_type_b", "/sys/pvs/usb_erase_count", "/ess/config/dcm/control_param/min_customer_soc", "/ess/config/dcm/mode_param/control_mode", "/net/wan0/state"} {
		if !slices.Contains(storage, v) {
			t.Errorf("PVS6 Storage: %s not read", v)
		}
	}

	// sys_type reads Unknown until the PVS sets it
	unknown := ExtraVars(map[string]string{"/sys/info/model": "PVS6", "/sys/info/sys_type": "Unknown"})
	if slices.Contains(unknown, "/ess/config/dcm/control_param/min_customer_soc") {
		t.Error("PVS6 Unknown: ESS config read")
	}

	pvs5 := ExtraVars(map[string]string{"/sys/info/model": "PVS5", "/sys/info/sys_type": "PV-only"})
	for _, v := range []string{"/sys/pvs/flashwear_type_b", "/sys/pvs/usb_erase_count", "/net/wan0/state"} {
		if slices.Contains(pvs5, v) {
			t.Errorf("PVS5: %s read", v)
		}
	}
	if !slices.Contains(pvs5, "/net/plc/state") {
		t.Error("PVS5: /net/plc/state not read")
	}
}
