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

// SerialParam is a device's serial number.
const SerialParam = "sn"

// LiveURL is the URL of the live data websocket of the PVS at host.
func LiveURL(host string) string { return fmt.Sprintf("ws://%s:%d", host, livePort) }
