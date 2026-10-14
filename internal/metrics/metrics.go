// Package metrics turns PVS variables into Prometheus samples and renders them
// in the Prometheus text format.
package metrics

import "github.com/cafedomingo/pvs_exporter/internal/pvs"

// PVS units to Prometheus base units.
const (
	kilo    = 1000 // kW, kVA, kvar to W, VA, var
	percent = 0.01
	minutes = 60
)

// field maps one PVS reading to a metric.
type field struct {
	metric string
	key    string
	scale  float64
	labels []Label
}

// infoField fills a label from a string param or variable.
type infoField struct{ label, key string }

var (
	leg1 = []Label{{"leg", "1"}}
	leg2 = []Label{{"leg", "2"}}
)

func flow(name string) []Label { return []Label{{"flow", name}} }

var gatewayFields = []field{
	{"pvs_gateway_cpu_usage_ratio", pvs.CPUUsageVar, percent, nil},
	{"pvs_gateway_ram_usage_ratio", pvs.RAMUsageVar, percent, nil},
	{"pvs_gateway_flash_usage_ratio", pvs.FlashUsageVar, percent, nil},
	{"pvs_gateway_flash_wear_ratio", pvs.FlashWearVar, 1, nil},
	{"pvs_gateway_uptime_seconds", pvs.UptimeVar, 1, nil},
	{"pvs_gateway_usb_erase_count", pvs.USBEraseVar, 1, nil},
	{"pvs_gateway_broadband_connected", pvs.BroadbandVar, 1, nil},
	{"pvs_gateway_cell_connected", pvs.CellVar, 1, nil},
	{"pvs_gateway_low_data_mode", pvs.LowDataVar, 1, nil},
	{"pvs_gateway_ess_reserve_soc_ratio", pvs.ReserveSOCVar, 1, nil},
	{"pvs_live_backup_time_remaining_seconds", pvs.BackupTimeVar, minutes, nil},
	{"pvs_live_mid_state", pvs.MidStateVar, 1, nil},
}

var gatewayInfo = []infoField{
	{"active_interface", pvs.ActiveInterfaceVar},
	{"board_type", pvs.BoardTypeVar},
	{"ess_control_mode", pvs.ControlModeVar},
	{"firmware_revision", pvs.FWRevVar},
	{"pvs_type", pvs.ModelVar},
	{"software_version", pvs.SWRevVar},
	{"sys_type", pvs.SysTypeVar},
}

// deviceKinds orders the output.
var deviceKinds = []string{"ess", "inverter", "meter", "transfer_switch"}

var deviceFields = map[string][]field{
	"ess": {
		{"pvs_ess_battery_voltage_volts", "vBattV", 1, nil},
		{"pvs_ess_cell_temperature_max_celsius", "maxTBattCellDegc", 1, nil},
		{"pvs_ess_cell_temperature_min_celsius", "minTBattCellDegc", 1, nil},
		{"pvs_ess_cell_voltage_max_volts", "maxVBattCellV", 1, nil},
		{"pvs_ess_cell_voltage_min_volts", "minVBattCellV", 1, nil},
		// the PVS counts energy into the battery as negative
		{"pvs_ess_charge_energy_kwh_total", "negLtea3phsumKwh", 1, nil},
		{"pvs_ess_discharge_energy_kwh_total", "posLtea3phsumKwh", 1, nil},
		{"pvs_ess_charge_limit_watts", "chrgLimitPmaxKw", kilo, nil},
		{"pvs_ess_discharge_limit_watts", "dischrgLimPmaxKw", kilo, nil},
		{"pvs_ess_customer_soc_ratio", "customerSocVal", 1, nil},
		{"pvs_ess_soc_ratio", "socVal", 1, nil},
		{"pvs_ess_soh_ratio", "sohVal", 1, nil},
		{"pvs_ess_inverter_temperature_celsius", "tInvtrDegc", 1, nil},
		{"pvs_ess_leg_voltage_volts", "v1nV", 1, leg1},
		{"pvs_ess_leg_voltage_volts", "v2nV", 1, leg2},
		{"pvs_ess_power_watts", "p3phsumKw", kilo, nil},
		{"pvs_ess_last_report_timestamp_seconds", pvs.ReportTimeParam, 1, nil},
	},
	"inverter": {
		{"pvs_inverter_power_watts", "p3phsumKw", kilo, nil},
		{"pvs_inverter_energy_kwh_total", "ltea3phsumKwh", 1, nil},
		{"pvs_inverter_temperature_celsius", "tHtsnkDegc", 1, nil},
		{"pvs_inverter_voltage_volts", "vln3phavgV", 1, nil},
		{"pvs_inverter_current_amps", "i3phsumA", 1, nil},
		{"pvs_inverter_frequency_hertz", "freqHz", 1, nil},
		{"pvs_inverter_dc_power_watts", "pMppt1Kw", kilo, nil},
		{"pvs_inverter_dc_voltage_volts", "vMppt1V", 1, nil},
		{"pvs_inverter_dc_current_amps", "iMppt1A", 1, nil},
		{"pvs_inverter_last_report_timestamp_seconds", pvs.ReportTimeParam, 1, nil},
	},
	"meter": {
		{"pvs_meter_power_watts", "p3phsumKw", kilo, nil},
		{"pvs_meter_apparent_power_va", "s3phsumKva", kilo, nil},
		{"pvs_meter_reactive_power_var", "q3phsumKvar", kilo, nil},
		{"pvs_meter_power_factor_ratio", "totPfRto", 1, nil},
		{"pvs_meter_average_voltage_volts", "vln3phavgV", 1, nil},
		{"pvs_meter_line_voltage_volts", "v12V", 1, nil},
		{"pvs_meter_current_amps", "i3phsumA", 1, nil},
		{"pvs_meter_frequency_hertz", "freqHz", 1, nil},
		{"pvs_meter_ct_scale_factor", "ctSclFctr", 1, nil},
		{"pvs_meter_energy_kwh_total", "ltea3phsumKwh", 1, nil},
		{"pvs_meter_import_energy_kwh_total", "posLtea3phsumKwh", 1, nil},
		{"pvs_meter_export_energy_kwh_total", "negLtea3phsumKwh", 1, nil},
		{"pvs_meter_net_energy_kwh_total", "netLtea3phsumKwh", 1, nil},
		{"pvs_meter_leg_power_watts", "p1Kw", kilo, leg1},
		{"pvs_meter_leg_power_watts", "p2Kw", kilo, leg2},
		{"pvs_meter_leg_current_amps", "i1A", 1, leg1},
		{"pvs_meter_leg_current_amps", "i2A", 1, leg2},
		{"pvs_meter_leg_voltage_volts", "v1nV", 1, leg1},
		{"pvs_meter_leg_voltage_volts", "v2nV", 1, leg2},
		{"pvs_meter_last_report_timestamp_seconds", pvs.ReportTimeParam, 1, nil},
	},
	"transfer_switch": {
		{"pvs_transfer_switch_grid_voltage_volts", "v1nGridV", 1, leg1},
		{"pvs_transfer_switch_grid_voltage_volts", "v2nGridV", 1, leg2},
		{"pvs_transfer_switch_voltage_volts", "v1nV", 1, leg1},
		{"pvs_transfer_switch_voltage_volts", "v2nV", 1, leg2},
		{"pvs_transfer_switch_supply_voltage_volts", "vSpplyV", 1, nil},
		{"pvs_transfer_switch_temperature_celsius", "tDegc", 1, nil},
		{"pvs_transfer_switch_last_report_timestamp_seconds", pvs.ReportTimeParam, 1, nil},
	},
}

// deviceInfo exports string params as labels on a constant-1 metric.
var deviceInfo = map[string]struct {
	metric string
	fields []infoField
}{
	"ess":             {"pvs_ess_info", []infoField{{"op_mode", "opMode"}}},
	"transfer_switch": {"pvs_transfer_switch_info", []infoField{{"mid_state", "midStEnum"}, {"pvd1_state", "pvd1StEnum"}}},
}

var liveFields = []field{
	{"pvs_live_power_watts", "pv_p", kilo, flow("pv")},
	{"pvs_live_power_watts", "net_p", kilo, flow("net")},
	{"pvs_live_power_watts", "site_load_p", kilo, flow("load")},
	{"pvs_live_power_watts", "ess_p", kilo, flow("battery")},
	{"pvs_live_energy_kwh", "pv_en", 1, flow("pv")},
	{"pvs_live_energy_kwh", "net_en", 1, flow("net")},
	{"pvs_live_energy_kwh", "site_load_en", 1, flow("load")},
	{"pvs_live_energy_kwh", "ess_en", 1, flow("battery")},
	{"pvs_live_soc_ratio", "soc", 1, nil},
}

// parsers read the keys that are not plain numbers.
var parsers = map[string]func(string) (float64, bool){
	pvs.ReportTimeParam: reportTime,
	pvs.FlashWearVar:    flashWear,
	pvs.BroadbandVar:    flag,
	pvs.CellVar:         flag,
}
