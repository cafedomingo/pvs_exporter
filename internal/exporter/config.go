// Package exporter polls a PVS, serves its metrics, and pushes its live data.
package exporter

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cafedomingo/pvs_exporter/internal/metrics"
	"github.com/cafedomingo/pvs_exporter/internal/pvs"
)

// Tunables, set by how fast the PVS's data changes.
const (
	defaultListen  = ":9711"
	coreEvery      = 30 * time.Second // CPU, RAM and uptime move continuously
	deviceEvery    = time.Minute      // the PVS refreshes all devices every 5 minutes
	extrasEvery    = 5 * time.Minute
	requestTimeout = 20 * time.Second
	liveStall      = 10 * time.Second // the PVS sends a frame every second
	liveRetry      = 5 * time.Second
	pushEvery      = 10 * time.Second
	unsentMax      = time.Hour

	// a poll never runs into the next
	pollTimeout = coreEvery
	// half a poll of slack keeps tick jitter from pushing a read to the poll
	// after
	dueSlack = coreEvery / 2
	// the PVS sends one frame a second
	queueMax = int(unsentMax / time.Second)
	// frames waiting while a push, at most pushEvery, is in flight
	frameBuffer = 2 * int(pushEvery/time.Second)
)

// Config is what the environment sets.
type Config struct {
	Listen       string
	PVSHost      string
	PVSPassword  string
	LiveURL      string
	PushURL      string
	PushUser     string
	PushPassword string
	PushLabels   []metrics.Label
}

// ConfigFromEnv reads the configuration with getenv, e.g. os.Getenv.
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	c := Config{
		Listen:       getenv("LISTEN_ADDRESS"),
		PVSHost:      getenv("PVS_HOST"),
		PVSPassword:  getenv("PVS_PASSWORD"),
		PushURL:      getenv("PUSH_URL"),
		PushUser:     getenv("PUSH_USER"),
		PushPassword: getenv("PUSH_PASSWORD"),
	}
	if c.PVSHost == "" {
		return Config{}, errors.New("PVS_HOST is required")
	}
	if c.Listen == "" {
		c.Listen = defaultListen
	}
	c.LiveURL = pvs.LiveURL(c.PVSHost)
	for pair := range strings.SplitSeq(getenv("PUSH_LABELS"), ",") {
		if pair == "" {
			continue
		}
		name, value, ok := strings.Cut(pair, "=")
		if !ok || name == "" {
			return Config{}, fmt.Errorf("PUSH_LABELS: %q is not name=value", pair)
		}
		c.PushLabels = append(c.PushLabels, metrics.Label{Name: name, Value: value})
	}
	return c, nil
}
