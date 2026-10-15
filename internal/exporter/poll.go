package exporter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cafedomingo/pvs_exporter/internal/metrics"
	"github.com/cafedomingo/pvs_exporter/internal/pvs"
)

// poller reads the PVS at the pace its data changes, keeping the results of
// the slower reads between them.
type poller struct {
	client       *pvs.Client
	sysType      string // at the last extras read
	extras       map[string]string
	extrasAt     time.Time
	extrasOutage outage
	devices      []metrics.Sample
	devicesAt    time.Time
	identity     map[string]string // logged variables at the last poll
	uptime       float64
}

func newPoller(c *pvs.Client) *poller {
	return &poller{client: c, extrasOutage: outage{what: "reading the gateway extras"}}
}

// due reports whether a read made at last is due again at now.
func due(last, now time.Time, every time.Duration) bool {
	return now.Sub(last) > every-dueSlack
}

// poll reads the gateway, and the extras and devices when due, and returns
// every sample.
func (p *poller) poll(ctx context.Context, now time.Time) ([]metrics.Sample, error) {
	core, err := p.client.Vars(ctx, metrics.CoreVars...)
	if err != nil {
		return nil, fmt.Errorf("reading the gateway: %w", err)
	}
	p.notice(core)
	// sys_type changes after a reboot, and with it the values worth reading
	if sysType := core[pvs.SysTypeVar]; sysType != p.sysType || due(p.extrasAt, now, extrasEvery) {
		p.sysType, p.extrasAt = sysType, now
		if extras, err := p.client.Vars(ctx, metrics.ExtraVars(core)...); err != nil {
			p.extrasOutage.fail(err)
		} else {
			p.extrasOutage.ok()
			p.extras = extras
		}
	}
	if due(p.devicesAt, now, deviceEvery) {
		vars, err := p.client.Match(ctx, pvs.DevicesPrefix)
		// 400: no device listed yet, e.g. after a reboot
		if err != nil && !errors.Is(err, pvs.ErrBadRequest) {
			return nil, fmt.Errorf("reading the devices: %w", err)
		}
		p.devices, p.devicesAt = metrics.Devices(pvs.Devices(vars)), now
	}
	gateway := maps.Clone(core)
	maps.Copy(gateway, p.extras)
	return slices.Concat(metrics.Gateway(gateway), p.devices), nil
}

// identityVars are the gateway variables logged when first seen and when they
// change, named as pvs_gateway_info labels them: a firmware update can change
// how the PVS answers.
var identityVars = []struct{ key, name string }{
	{"pvs_type", pvs.ModelVar},
	{"software_version", pvs.SWRevVar},
	{"sys_type", pvs.SysTypeVar},
}

// notice logs the PVS when first seen, a restart when its uptime goes
// backwards, and then any change to the identity variables.
func (p *poller) notice(core map[string]string) {
	identity := map[string]string{}
	var found, changed []any
	for _, v := range identityVars {
		value := strings.TrimSpace(core[v.name])
		identity[v.key] = value
		found = append(found, v.key, value)
		if was := p.identity[v.key]; was != value {
			changed = append(changed, v.key, value, "was_"+v.key, was)
		}
	}
	if uptime, err := strconv.ParseFloat(strings.TrimSpace(core[pvs.UptimeVar]), 64); err == nil {
		up := time.Duration(uptime) * time.Second
		found = append(found, "uptime", up)
		if uptime < p.uptime {
			slog.Info("PVS restarted", "uptime", up)
		}
		p.uptime = uptime
	}
	switch {
	case p.identity == nil:
		slog.Info("PVS found", found...)
	case changed != nil:
		slog.Info("PVS changed", changed...)
	}
	p.identity = identity
}
