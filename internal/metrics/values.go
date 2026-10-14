package metrics

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cafedomingo/pvs_exporter/internal/pvs"
)

// number parses a varserver number. Varserver defaults such as "N/A" and
// "TBD" are not readings, and neither are NaN and infinity.
func number(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
}

// text trims a varserver string, reading pvs.Unset as empty.
func text(s string) string {
	s = strings.TrimSpace(s)
	if s == pvs.Unset {
		return ""
	}
	return s
}

// reportTime parses a device's msmtEps, e.g. "2026-10-14T08:25:00Z".
func reportTime(s string) (float64, bool) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
	return float64(t.Unix()), err == nil
}

// flashWear parses eMMC lifetime used, in hex tenths ("0x02" is 20%), as a
// ratio.
func flashWear(s string) (float64, bool) {
	tenths, err := strconv.ParseUint(text(s), 0, 64)
	return min(float64(tenths)/10, 1), err == nil
}

// flag parses a varserver boolean: any non-zero number is true.
func flag(s string) (float64, bool) {
	v, ok := number(s)
	if v != 0 {
		v = 1
	}
	return v, ok
}
