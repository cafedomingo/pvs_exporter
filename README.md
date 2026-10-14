# pvs_exporter

Prometheus exporter for SunStrong (formerly SunPower) PVS5 and PVS6 gateways:
the gateway itself, its inverters, meters, SunVault battery and transfer
switch, and the PVS's one-second live power data. One static binary, talking
to the PVS's local API.

It reads the PVS no faster than its data changes: the gateway every 30
seconds, devices every minute (the PVS refreshes them every 5), and rarely
changing settings every 5 minutes.

## Configuration

Environment variables:

| Variable | Meaning |
|---|---|
| `PVS_HOST` | PVS address, e.g. `192.168.1.10`. Required. |
| `PVS_PASSWORD` | Local API password. Default: the last 5 characters of the serial number, the PVS's own default. |
| `LISTEN_ADDRESS` | Where to serve `/metrics`. Default `:9711`. |
| `PUSH_URL` | Optional. Push the live data, timestamped by the PVS, to this URL instead of serving it on `/metrics`. |
| `PUSH_USER`, `PUSH_PASSWORD` | Optional HTTP Basic auth for `PUSH_URL`. |
| `PUSH_LABELS` | Labels added to pushed samples, e.g. `job=pvs,instance=pvs-exporter:9711`. |

Without `PUSH_URL`, the latest live values appear on `/metrics` at your scrape
interval. With it, every one-second sample is kept: the exporter POSTs
Prometheus text lines with timestamps every 10 seconds, and holds up to an
hour of them while the receiver is unreachable. VictoriaMetrics accepts them
at `/api/v1/import/prometheus`; set `PUSH_LABELS` to the `job` and `instance`
your scrape uses so the pushed series line up with the scraped ones.

## Metrics

All metrics are gauges, prefixed `pvs_`. Devices carry `serial` and `model`
labels, and per-leg values a `leg` label.
[internal/metrics/metrics.go](internal/metrics/metrics.go) maps each PVS reading
to its metric.

- `pvs_up`, `pvs_poll_duration_seconds`, `pvs_live_connected`: exporter status
- `pvs_gateway_*`: CPU, RAM and flash use, flash wear, uptime, connectivity,
  battery reserve; `pvs_gateway_info` and `pvs_gateway_interface_info` carry
  versions and link states as labels
- `pvs_live_*`: power and energy per flow (`pv`, `net`, `load`, `battery`),
  battery state of charge, backup time remaining
- `pvs_inverter_*`, `pvs_meter_*`, `pvs_ess_*`, `pvs_transfer_switch_*`:
  every value the devices report, plus `*_last_report_timestamp_seconds`

## License

MIT
