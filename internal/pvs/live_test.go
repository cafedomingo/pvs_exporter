package pvs_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cafedomingo/pvs_exporter/v2/internal/pvs"
	"github.com/cafedomingo/pvs_exporter/v2/internal/pvstest"
)

func TestStreamEmitsPowerFrames(t *testing.T) {
	url := pvstest.Live(t, 0,
		`{"notification": "power", "params": {"time": 100, "pv_p": 3.2, "soc": 0.68}}`,
		`{"notification": "power", "params": {"time": 100, "pv_p": 9.9}}`,
		`{"notification": "other", "params": {"state": "connected"}}`,
		`{"notification": "power", "params": {"pv_p": 1.0}}`, // no time
		`{"notification": "power", "params": {"time": 102, "pv_p": 3.3}}`,
	)
	var got []pvs.Frame

	start := time.Now()
	err := pvs.Stream(t.Context(), url, 100*time.Millisecond, func(f pvs.Frame) { got = append(got, f) })

	want := []pvs.Frame{
		{Time: 100, Params: map[string]float64{"pv_p": 3.2, "soc": 0.68}},
		{Time: 100, Params: map[string]float64{"pv_p": 9.9}},
		{Time: 102, Params: map[string]float64{"pv_p": 3.3}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("frames = %v, want %v", got, want)
	}
	// the server goes silent after its frames, which must count as a stall
	if err == nil || time.Since(start) > time.Second {
		t.Errorf("Stream returned %v after %v, want a stall error after about 100ms", err, time.Since(start))
	}
}

func TestStreamFailsOnAFrameItCannotDecode(t *testing.T) {
	url := pvstest.Live(t, 0,
		`{"notification": "power", "params": {"time": 100, "pv_p": 3.2}}`,
		`{"notification": "power", "params": {"time": 101, "pv_p": "3.3"}}`,
	)
	n := 0

	err := pvs.Stream(t.Context(), url, time.Second, func(pvs.Frame) { n++ })

	if n != 1 || err == nil || !strings.Contains(err.Error(), "decoding") {
		t.Errorf("got %d frames and %v, want 1 frame and a decoding error", n, err)
	}
}

func TestStreamOutlivesItsConnectTimeout(t *testing.T) {
	url := pvstest.Live(t, 50*time.Millisecond,
		`{"notification": "power", "params": {"time": 1}}`,
		`{"notification": "power", "params": {"time": 2}}`,
		`{"notification": "power", "params": {"time": 3}}`,
		`{"notification": "power", "params": {"time": 4}}`,
	)
	n := 0

	err := pvs.Stream(t.Context(), url, 100*time.Millisecond, func(pvs.Frame) { n++ })

	if err == nil {
		t.Error("Stream did not report the stall after the last frame")
	}
	if n != 4 {
		t.Errorf("got %d frames over 200ms with a 100ms stall, want 4", n)
	}
}

func TestStreamFailsWhenNothingListens(t *testing.T) {
	err := pvs.Stream(t.Context(), "ws://127.0.0.1:1", 100*time.Millisecond, func(pvs.Frame) {})
	if err == nil {
		t.Fatal("Stream succeeded with nothing listening")
	}
}
