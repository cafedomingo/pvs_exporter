package metrics

import "github.com/cafedomingo/pvs_exporter/internal/pvs"

// Live returns the samples for one live data frame.
func Live(f pvs.Frame) []Sample {
	var out []Sample
	for _, fl := range liveFields {
		if v, ok := f.Params[fl.key]; ok {
			out = append(out, Sample{fl.metric, fl.labels, v * fl.scale})
		}
	}
	return append(out, Sample{"pvs_live_timestamp_seconds", nil, float64(f.Time)})
}
