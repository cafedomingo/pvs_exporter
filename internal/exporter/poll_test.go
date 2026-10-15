package exporter

import (
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cafedomingo/pvs_exporter/v2/internal/metrics"
	"github.com/cafedomingo/pvs_exporter/v2/internal/pvs"
	"github.com/cafedomingo/pvs_exporter/v2/internal/pvstest"
)

// gateway is a PVS6 (Storage) with every core and extra variable.
var gateway = map[string]string{
	"/sys/info/cpu_usage": "11", "/sys/info/flash_usage": "73", "/sys/info/hwrev": "6.02",
	"/sys/info/model": "PVS6", "/sys/info/ram_usage": "7", "/sys/info/sw_rev": "2025.10.20.61846",
	"/sys/info/sys_type": "Storage", "/sys/info/uptime": "4201.33",
	"/sys/livedata/backupTimeRemaining": "167", "/sys/livedata/midstate": "2",
	"/sys/info/active_interface": "wan0", "/sys/info/boardtype": "A0K LIB 1.0 PSoC5", "/sys/info/fwrev": "",
	"/sys/toggle_cell/broadband_connected": "1", "/sys/toggle_cell/cell_connected": "0",
	"/sys/toggle_cell/low_data_mode": "0", "/sys/pvs/flashwear_type_b": "0x01", "/sys/pvs/usb_erase_count": "0",
	"/ess/config/dcm/control_param/min_customer_soc": "0.20",
	"/ess/config/dcm/mode_param/control_mode":        "SELF_CONSUMPTION",
	"/net/sta0/state": "ready", "/net/wan0/state": "online", "/net/wan1/state": "N/A", "/net/wwan0/state": "N/A",
	"/sys/telemetryws/enable": "1",
}

var meter = map[string]string{
	"/sys/devices/meter/0/sn":        "FAKE-METER-0",
	"/sys/devices/meter/0/p3phsumKw": "0.71",
}

const (
	coreQuery   = "name=/sys/info/cpu_usage"
	extrasQuery = "name=/sys/info/active_interface"
	deviceQuery = "match=/sys/devices/"
)

// reads names each query by its first variable, dropping the login's serial read.
func reads(queries []string) []string {
	var out []string
	for _, q := range queries {
		q, _, _ = strings.Cut(q, "&")
		q, _, _ = strings.Cut(q, ",")
		if q != "name=/sys/info/serialnum" {
			out = append(out, q)
		}
	}
	return out
}

func TestPollReadsAtThePaceTheDataChanges(t *testing.T) {
	fake := pvstest.New(t, gateway)
	fake.Set(meter)
	p := newPoller(pvs.NewClient(fake.Host(), "", time.Second))
	t0 := time.Now()

	for _, step := range []struct {
		at   time.Duration
		want []string
	}{
		{0, []string{coreQuery, extrasQuery, deviceQuery}},
		{30 * time.Second, []string{coreQuery}},
		{60 * time.Second, []string{coreQuery, deviceQuery}},
		{90 * time.Second, []string{coreQuery}},
		// a tick a little early still reads the devices
		{120*time.Second - 10*time.Millisecond, []string{coreQuery, deviceQuery}},
		{300 * time.Second, []string{coreQuery, extrasQuery, deviceQuery}},
	} {
		if _, err := p.poll(t.Context(), t0.Add(step.at)); err != nil {
			t.Fatalf("at %v: %v", step.at, err)
		}
		if got := reads(fake.Queries()); !slices.Equal(got, step.want) {
			t.Errorf("at %v: reads = %q, want %q", step.at, got, step.want)
		}
	}
}

func TestPollRereadsExtrasWhenTheSystemTypeChanges(t *testing.T) {
	// sys_type reads Unknown for a while after a reboot
	for _, sysType := range []string{"Storage", "PV-only"} {
		fake := pvstest.New(t, gateway)
		fake.Set(map[string]string{"/sys/info/sys_type": "Unknown"})
		p := newPoller(pvs.NewClient(fake.Host(), "", time.Second))
		t0 := time.Now()
		if _, err := p.poll(t.Context(), t0); err != nil {
			t.Fatal(err)
		}
		fake.Queries()
		fake.Set(map[string]string{"/sys/info/sys_type": sysType})

		if _, err := p.poll(t.Context(), t0.Add(30*time.Second)); err != nil {
			t.Fatal(err)
		}
		if got := reads(fake.Queries()); !slices.Contains(got, extrasQuery) {
			t.Errorf("Unknown to %s: reads = %q, want the extras", sysType, got)
		}
	}
}

func TestPollWithoutDevicesSucceeds(t *testing.T) {
	fake := pvstest.New(t, gateway) // no devices: the match answers 400
	p := newPoller(pvs.NewClient(fake.Host(), "", time.Second))
	t0 := time.Now()

	samples, err := p.poll(t.Context(), t0)
	if err != nil {
		t.Fatal(err)
	}
	if hasMetric(samples, "pvs_meter_power_watts") {
		t.Error("meter samples without a meter")
	}

	fake.Set(meter)
	samples, err = p.poll(t.Context(), t0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !hasMetric(samples, "pvs_meter_power_watts") {
		t.Error("no meter samples once the meter appears")
	}
}

func TestPollFailsWhenTheGatewayIsUnreachable(t *testing.T) {
	fake := pvstest.New(t, gateway)
	fake.Set(meter)
	p := newPoller(pvs.NewClient(fake.Host(), "", time.Second))
	t0 := time.Now()
	if _, err := p.poll(t.Context(), t0); err != nil {
		t.Fatal(err)
	}
	fake.Close()

	samples, err := p.poll(t.Context(), t0.Add(30*time.Second))
	if err == nil {
		t.Fatal("poll succeeded with the PVS down")
	}
	// /metrics then carries only the exporter status, so series go stale
	if samples != nil {
		t.Errorf("a failed poll returned %d samples, want none", len(samples))
	}
}

func TestPollKeepsExtrasWhenTheirReadFails(t *testing.T) {
	fake := pvstest.New(t, gateway)
	p := newPoller(pvs.NewClient(fake.Host(), "", time.Second))
	t0 := time.Now()
	if _, err := p.poll(t.Context(), t0); err != nil {
		t.Fatal(err)
	}
	fake.Delete("/net/wwan0/state") // the next extras read answers 400
	fake.Queries()

	samples, err := p.poll(t.Context(), t0.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("a failed extras read failed the poll: %v", err)
	}
	if !slices.Contains(reads(fake.Queries()), extrasQuery) {
		t.Fatal("the extras were not read")
	}
	if !hasMetric(samples, "pvs_gateway_flash_wear_ratio") {
		t.Error("extras dropped after a failed read")
	}
}

func TestPollClearsDevicesThePVSNoLongerLists(t *testing.T) {
	fake := pvstest.New(t, gateway)
	fake.Set(meter)
	p := newPoller(pvs.NewClient(fake.Host(), "", time.Second))
	t0 := time.Now()
	if _, err := p.poll(t.Context(), t0); err != nil {
		t.Fatal(err)
	}
	fake.Delete(slices.Collect(maps.Keys(meter))...)

	samples, err := p.poll(t.Context(), t0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if hasMetric(samples, "pvs_meter_power_watts") {
		t.Error("meter samples kept after the meter disappeared")
	}
}

func TestPollLogsThePVSWhenFoundChangedOrRestarted(t *testing.T) {
	logs := captureLogs(t)
	fake := pvstest.New(t, gateway)
	p := newPoller(pvs.NewClient(fake.Host(), "", time.Second))
	t0 := time.Now()
	if _, err := p.poll(t.Context(), t0); err != nil {
		t.Fatal(err)
	}
	if _, err := p.poll(t.Context(), t0.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	fake.Set(map[string]string{"/sys/info/sw_rev": "2025.11.03.61900", "/sys/info/uptime": "211.5"})
	if _, err := p.poll(t.Context(), t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	var got []string
	for line := range strings.Lines(logs.String()) {
		if _, msg, ok := strings.Cut(line, "msg="); ok && strings.HasPrefix(msg, `"PVS `) {
			got = append(got, strings.TrimSpace(msg))
		}
	}
	want := []string{
		`"PVS found" pvs_type=PVS6 software_version=2025.10.20.61846 sys_type=Storage uptime=1h10m1s`,
		`"PVS restarted" uptime=3m31s`,
		`"PVS changed" software_version=2025.11.03.61900 was_software_version=2025.10.20.61846`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("PVS logs:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func hasMetric(samples []metrics.Sample, name string) bool {
	return slices.ContainsFunc(samples, func(s metrics.Sample) bool { return s.Name == name })
}
