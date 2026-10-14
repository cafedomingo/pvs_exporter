package pvs

import (
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Device is one device's parameters, e.g. "p3phsumKw": "0.71".
type Device map[string]string

// Devices groups /sys/devices/<kind>/<index>/<param> variables by kind, each
// kind in index order. Devices without a serial number are dropped.
func Devices(vars map[string]string) map[string][]Device {
	found := map[string]map[int]Device{}
	for name, value := range vars {
		rest, ok := strings.CutPrefix(name, DevicesPrefix)
		parts := strings.Split(rest, "/")
		if !ok || len(parts) != 3 {
			continue
		}
		index, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		kind, param := parts[0], parts[2]
		if found[kind] == nil {
			found[kind] = map[int]Device{}
		}
		if found[kind][index] == nil {
			found[kind][index] = Device{}
		}
		found[kind][index][param] = value
	}
	devices := map[string][]Device{}
	for kind, byIndex := range found {
		for _, index := range slices.Sorted(maps.Keys(byIndex)) {
			if d := byIndex[index]; d[SerialParam] != "" {
				devices[kind] = append(devices[kind], d)
			}
		}
	}
	return devices
}
