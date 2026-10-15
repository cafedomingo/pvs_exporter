# pvs_exporter

Prometheus exporter for SunStrong (formerly SunPower) PVS5 and PVS6 gateways:
the gateway itself, its inverters, meters, SunVault battery and transfer
switch, and the PVS's one-second live power data. One static binary, talking
to the PVS's local API.

It reads the PVS no faster than its data changes: the gateway every 30
seconds, devices every minute (the PVS refreshes them every 5), and rarely
changing settings every 5 minutes.

## Setup

You need the PVS's address on your network (your router's client list shows
it) and a host that can reach it. The local API needs PVS6 software build
61840 or later, or PVS5 build 5412 or later.

1. Run the exporter, here with Docker Compose. The image runs on linux amd64,
   arm64, armv7 and riscv64.

   ```yaml
   services:
     pvs-exporter:
       image: ghcr.io/cafedomingo/pvs_exporter:2
       restart: unless-stopped
       ports: ["9711:9711"]
       environment:
         PVS_HOST: 192.168.1.10
   ```

   `:2` follows every 2.x release; a new major version means a breaking
   change, such as renamed metrics. Without Docker, download the binary for
   your platform from the
   [releases page](https://github.com/cafedomingo/pvs_exporter/releases) and
   run it with the same environment, or
   `go install github.com/cafedomingo/pvs_exporter/v2/cmd/pvs_exporter@latest`.

2. Check it: `curl -s localhost:9711/metrics | grep ^pvs_up` shows
   `pvs_up 1` once the first poll succeeds.

3. Scrape it. 30 seconds matches how often the gateway is read; only the
   live values change faster, and `PUSH_URL` keeps every second of those:

   ```yaml
   scrape_configs:
     - job_name: pvs
       scrape_interval: 30s
       static_configs:
         - targets: ["pvs-exporter:9711"]
   ```

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
| `LOG_LEVEL` | `debug`, `info`, `warn` or `error`. Default `info`. |

Without `PUSH_URL`, the latest live values appear on `/metrics` at your scrape
interval. With it, every one-second sample is kept: the exporter POSTs
Prometheus text lines with timestamps every 10 seconds, and holds up to an
hour of them while the receiver is unreachable. VictoriaMetrics accepts them
at `/api/v1/import/prometheus`; set `PUSH_LABELS` to the `job` and `instance`
your scrape uses so the pushed series line up with the scraped ones.

## Metrics

All metrics are prefixed `pvs_`. Those ending in `_total` are counters and
the rest are gauges. Devices carry `serial` and `model` labels, and per-leg
values a `leg` label.
[internal/metrics/metrics.go](internal/metrics/metrics.go) maps each PVS reading
to its metric.

- `pvs_up`, `pvs_poll_duration_seconds`, `pvs_live_connected`: exporter status
- `pvs_exporter_build_info`, `pvs_live_frames_total`, and
  `pvs_requests_total` and `pvs_request_failures_total` by `request` (`login`,
  `name`, `match`, `set`): exporter stats. The PVS's 400 for an unknown name or
  an empty match, and its 401 for an expired session, don't count as failures.
  With `PUSH_URL`, also `pvs_push_queued_frames` and
  `pvs_push_dropped_frames_total`.
- `pvs_gateway_*`: CPU, RAM and flash use, flash wear, uptime, connectivity,
  battery reserve; `pvs_gateway_info` and `pvs_gateway_interface_info` carry
  versions and link states as labels
- `pvs_live_*`: power and energy per flow (`pv`, `net`, `load`, `battery`),
  battery state of charge, backup time remaining
- `pvs_inverter_*`, `pvs_meter_*`, `pvs_ess_*`, `pvs_transfer_switch_*`:
  every value the devices report, plus `*_last_report_timestamp_seconds`

## Development

```sh
go test -race ./...
golangci-lint run
PVS_HOST=192.168.1.10 go run ./cmd/pvs_exporter
```

The tests run against a fake PVS (`internal/pvstest`) that answers the way a
real one was measured to, so they need no hardware. `internal/pvs` is the
client for the local API and live websocket, `internal/metrics` maps PVS
variables to metrics, and `internal/exporter` polls, serves and pushes; its
`config.go` holds every interval and limit. [docs/pvs-api.md](docs/pvs-api.md)
describes the PVS API as the exporter uses it.

To release, push a `vX.Y.Z` tag; don't draft the release on GitHub, since
the workflow creates it. The workflow tests, builds the binaries onto a
GitHub release, and pushes the image to ghcr.io. Go puts the major version in
the module path, so a new major version also changes it in `go.mod` and every
import.
