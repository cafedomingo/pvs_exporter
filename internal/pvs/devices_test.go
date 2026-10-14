package pvs_test

import (
	"reflect"
	"testing"

	"github.com/cafedomingo/pvs_exporter/internal/pvs"
)

func TestDevicesGroupsByKindInIndexOrder(t *testing.T) {
	vars := map[string]string{
		"/sys/devices/meter/10/sn":        "M10",
		"/sys/devices/meter/10/p3phsumKw": "0.7",
		"/sys/devices/meter/2/sn":         "M2",
		"/sys/devices/ess/0/sn":           "E0",
		"/sys/devices/ess/0/socVal":       "0.32",
		"/sys/devices/ess/1/socVal":       "0.5", // no serial: dropped
		"/sys/devices/meter/x/sn":         "bad index",
		"/sys/devices/meter/3":            "too short",
	}

	want := map[string][]pvs.Device{
		"meter": {{"sn": "M2"}, {"sn": "M10", "p3phsumKw": "0.7"}},
		"ess":   {{"sn": "E0", "socVal": "0.32"}},
	}
	if got := pvs.Devices(vars); !reflect.DeepEqual(got, want) {
		t.Errorf("Devices = %v, want %v", got, want)
	}
}
