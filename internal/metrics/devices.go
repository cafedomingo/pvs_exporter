package metrics

import (
	"slices"

	"github.com/cafedomingo/pvs_exporter/v2/internal/pvs"
)

// Devices returns the samples for every device, by kind and then index.
func Devices(devices map[string][]pvs.Device) []Sample {
	var out []Sample
	for _, kind := range deviceKinds {
		for _, d := range devices[kind] {
			id := []Label{{"serial", d[pvs.SerialParam]}, {"model", text(d[pvs.ModelParam])}}
			out = appendFields(out, deviceFields[kind], d, id)
			if info, ok := deviceInfo[kind]; ok {
				out = append(out, infoSample(info.metric, info.fields, d, id))
			}
		}
	}
	return out
}

// appendFields appends a sample for each field whose key holds a reading.
func appendFields(out []Sample, fields []field, values map[string]string, labels []Label) []Sample {
	for _, f := range fields {
		parse := parsers[f.key]
		if parse == nil {
			parse = number
		}
		if v, ok := parse(values[f.key]); ok {
			out = append(out, Sample{Name: f.metric, Labels: slices.Concat(labels, f.labels), Value: v * f.scale})
		}
	}
	return out
}

// infoSample is a constant-1 sample carrying string values as labels.
func infoSample(metric string, fields []infoField, values map[string]string, labels []Label) Sample {
	labels = slices.Clone(labels)
	for _, f := range fields {
		labels = append(labels, Label{f.label, text(values[f.key])})
	}
	return Sample{Name: metric, Labels: labels, Value: 1}
}
