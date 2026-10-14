package metrics

import "github.com/cafedomingo/pvs_exporter/internal/pvs"

// CoreVars are read every poll. Each exists on the PVS5 and the PVS6, so the
// batch never fails for an unknown name.
var CoreVars = []string{
	pvs.CPUUsageVar, pvs.RAMUsageVar, pvs.FlashUsageVar, pvs.UptimeVar,
	pvs.ModelVar, pvs.HWRevVar, pvs.SWRevVar, pvs.SysTypeVar,
	pvs.BackupTimeVar, pvs.MidStateVar,
}

// ExtraVars are the rarely changing variables this PVS has, chosen by its
// model and system type in a core read.
func ExtraVars(core map[string]string) []string {
	names := []string{pvs.ActiveInterfaceVar, pvs.BoardTypeVar, pvs.FWRevVar, pvs.BroadbandVar, pvs.CellVar, pvs.LowDataVar}
	if !pvs5(core) {
		names = append(names, pvs.FlashWearVar, pvs.USBEraseVar)
		if text(core[pvs.SysTypeVar]) == pvs.SysTypeStorage {
			names = append(names, pvs.ReserveSOCVar, pvs.ControlModeVar)
		}
	}
	for _, i := range interfaces(core) {
		names = append(names, pvs.NetStateVar(i))
	}
	return names
}

// Gateway returns the gateway samples, and the live values polled with them,
// from the core and extra variables.
func Gateway(vars map[string]string) []Sample {
	out := appendFields(nil, gatewayFields, vars, nil)
	info := infoSample("pvs_gateway_info", gatewayInfo, vars, nil)
	hardware := text(vars[pvs.ModelVar]) + " " + text(vars[pvs.HWRevVar])
	info.Labels = append(info.Labels, Label{"hardware_version", hardware})
	out = append(out, info)
	for _, i := range interfaces(vars) {
		if state := text(vars[pvs.NetStateVar(i)]); state != "" {
			out = append(out, Sample{"pvs_gateway_interface_info", []Label{{"interface", i}, {"state", state}}, 1})
		}
	}
	return out
}

func pvs5(vars map[string]string) bool { return text(vars[pvs.ModelVar]) == pvs.ModelPVS5 }

func interfaces(vars map[string]string) []string { return pvs.Interfaces(text(vars[pvs.ModelVar])) }
