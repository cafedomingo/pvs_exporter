package metrics

import "github.com/cafedomingo/pvs_exporter/v2/internal/pvs"

// Live returns the samples for one live data frame.
func Live(f pvs.Frame) []Sample {
	var out []Sample
	for _, fl := range liveFields {
		if v, ok := f.Params[fl.key]; ok {
			out = append(out, Sample{Name: fl.metric, Labels: fl.labels, Value: v * fl.scale})
		}
	}
	return append(out, Sample{Name: "pvs_live_timestamp_seconds", Value: float64(f.Time)})
}
