package metrics

import (
	"testing"
	"time"
)

func TestParsers(t *testing.T) {
	report := float64(time.Date(2026, 10, 14, 8, 25, 0, 0, time.UTC).Unix())
	tests := []struct {
		name  string
		parse func(string) (float64, bool)
		in    string
		want  float64
		ok    bool
	}{
		{"number", number, "0.320000", 0.32, true},
		{"number spaces", number, " 5 ", 5, true},
		{"number N/A", number, "N/A", 0, false},
		{"number TBD", number, "TBD", 0, false},
		{"number empty", number, "", 0, false},
		{"number NaN", number, "NaN", 0, false},
		{"number Inf", number, "+Inf", 0, false},
		{"report time", reportTime, "2026-10-14T08:25:00Z", report, true},
		{"report time empty", reportTime, "", 0, false},
		{"flash wear", flashWear, "0x01", 0.1, true},
		{"flash wear no prefix", flashWear, "02", 0.2, true},
		{"flash wear capped", flashWear, "0x0B", 1, true},
		{"flash wear upper case", flashWear, "0X05", 0.5, true},
		{"flash wear TBD", flashWear, "TBD", 0, false},
		{"flag on", flag, "1", 1, true},
		{"flag off", flag, "0", 0, true},
		{"flag N/A", flag, "N/A", 0, false},
	}
	for _, tt := range tests {
		got, ok := tt.parse(tt.in)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("%s(%q) = %v, %v; want %v, %v", tt.name, tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestText(t *testing.T) {
	for in, want := range map[string]string{" Major 2 ": "Major 2", "TBD": "", "N/A": "N/A", "": ""} {
		if got := text(in); got != want {
			t.Errorf("text(%q) = %q, want %q", in, got, want)
		}
	}
}
