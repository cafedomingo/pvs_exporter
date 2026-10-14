package pvs

import "fmt"

// The local API's paths, query keys and the variables the client needs.
const (
	loginPath = "/auth?login"
	varsPath  = "/vars"
	loginUser = "ssm_owner"
	// the default password is the serial number's last passwordLen characters
	passwordLen = 5

	nameQuery  = "name="
	matchQuery = "match="
	setQuery   = "set="
	objFormat  = "&fmt=obj" // one flat JSON object of name to value

	serialVar = "/sys/info/serialnum"
	liveVar   = "/sys/telemetryws/enable"

	livePort          = 9002
	powerNotification = "power"
	timeParam         = "time"
)

// DevicesPrefix starts the name of every device variable,
// DevicesPrefix<kind>/<index>/<param>.
const DevicesPrefix = "/sys/devices/"

// Device params that identify a device or date its readings.
const (
	SerialParam     = "sn"
	ModelParam      = "prodMdlNm"
	ReportTimeParam = "msmtEps" // when the PVS measured the device
)

// Gateway variables.
const (
	CPUUsageVar        = "/sys/info/cpu_usage"
	RAMUsageVar        = "/sys/info/ram_usage"
	FlashUsageVar      = "/sys/info/flash_usage"
	UptimeVar          = "/sys/info/uptime"
	ModelVar           = "/sys/info/model"
	HWRevVar           = "/sys/info/hwrev"
	SWRevVar           = "/sys/info/sw_rev"
	FWRevVar           = "/sys/info/fwrev"
	BoardTypeVar       = "/sys/info/boardtype"
	ActiveInterfaceVar = "/sys/info/active_interface"
	SysTypeVar         = "/sys/info/sys_type" // "PV-only" or "Storage"; "Unknown" until the PVS sets it
	BackupTimeVar      = "/sys/livedata/backupTimeRemaining"
	MidStateVar        = "/sys/livedata/midstate"
	BroadbandVar       = "/sys/toggle_cell/broadband_connected"
	CellVar            = "/sys/toggle_cell/cell_connected"
	LowDataVar         = "/sys/toggle_cell/low_data_mode"
	FlashWearVar       = "/sys/pvs/flashwear_type_b"
	USBEraseVar        = "/sys/pvs/usb_erase_count"
	ReserveSOCVar      = "/ess/config/dcm/control_param/min_customer_soc"
	ControlModeVar     = "/ess/config/dcm/mode_param/control_mode"
)

// Values of the gateway variables that decide what else to read.
const (
	ModelPVS5      = "PVS5"
	SysTypeStorage = "Storage"
)

// Unset is a string variable's value until something sets it.
const Unset = "TBD"

// Interfaces are the network interfaces a PVS of model reports link states
// for; anything but a PVS5 is taken for a PVS6.
func Interfaces(model string) []string {
	if model == ModelPVS5 {
		return []string{"cell", "plc", "sta0", "wan"}
	}
	return []string{"sta0", "wan0", "wan1", "wwan0"}
}

// NetStateVar names iface's link state variable.
func NetStateVar(iface string) string { return "/net/" + iface + "/state" }

// LiveURL is the URL of the live data websocket of the PVS at host.
func LiveURL(host string) string { return fmt.Sprintf("ws://%s:%d", host, livePort) }
