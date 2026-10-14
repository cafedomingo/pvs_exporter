package metrics

import (
	"slices"
	"strconv"
	"strings"
)

// Label is one metric label.
type Label struct{ Name, Value string }

// Sample is one metric value. Every metric is a gauge.
type Sample struct {
	Name   string
	Labels []Label
	Value  float64
}

// ContentType is the media type of the text format Format and AppendPush
// write.
const ContentType = "text/plain; version=0.0.4; charset=utf-8"

var escape = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)

// Format renders samples for a scrape: grouped by name, each name led by its
// TYPE line.
func Format(samples []Sample) []byte {
	sorted := slices.Clone(samples)
	slices.SortStableFunc(sorted, func(a, b Sample) int { return strings.Compare(a.Name, b.Name) })
	var b []byte
	for i, s := range sorted {
		if i == 0 || s.Name != sorted[i-1].Name {
			b = append(b, "# TYPE "+s.Name+" gauge\n"...)
		}
		b = appendSample(b, s, nil, 0)
	}
	return b
}

// AppendPush appends samples as push lines, each with the extra labels and
// stamped with ms, Unix milliseconds.
func AppendPush(b []byte, samples []Sample, extra []Label, ms int64) []byte {
	for _, s := range samples {
		b = appendSample(b, s, extra, ms)
	}
	return b
}

func appendSample(b []byte, s Sample, extra []Label, ms int64) []byte {
	b = append(b, s.Name...)
	for i, l := range slices.Concat(s.Labels, extra) {
		if i == 0 {
			b = append(b, '{')
		} else {
			b = append(b, ',')
		}
		b = append(b, l.Name+`="`+escape.Replace(l.Value)+`"`...)
	}
	if len(s.Labels)+len(extra) > 0 {
		b = append(b, '}')
	}
	b = append(b, ' ')
	b = strconv.AppendFloat(b, s.Value, 'g', -1, 64)
	if ms != 0 {
		b = append(b, ' ')
		b = strconv.AppendInt(b, ms, 10)
	}
	return append(b, '\n')
}
