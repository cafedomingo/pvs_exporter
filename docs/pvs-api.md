# The PVS local API, as pvs_exporter uses it

SunStrong documents the API in
[pypvs's LocalAPI.md](https://github.com/cafedomingo/pypvs/blob/main/doc/LocalAPI.md),
with every variable in its
[PVS5](https://github.com/cafedomingo/pypvs/blob/main/doc/varserver-variables-public-pvs5.csv)
and [PVS6](https://github.com/cafedomingo/pypvs/blob/main/doc/varserver-variables-public-pvs6.csv)
lists. This page covers the part the exporter uses, and how the PVS behaves
where those documents are silent.

## Requests

The PVS serves HTTPS on its LAN address with a self-signed certificate.

**Login.** `GET /auth?login` with HTTP Basic auth, user `ssm_owner`. The
default password is the last 5 characters of the PVS serial number. The PVS
answers with a `session` cookie for later requests. Without a session it
answers 400 (errorcode `0x0100`); with a session it does not know, 401
(errorcode `0x0012`), so a 401 means log in again.

**Serial number.** `/sys/info/serialnum` reads without a session, so the
default password can be derived before logging in.

**Reads.** `POST /vars` with a form body, not URL-encoded, since names are
paths:

- `name=/sys/info/uptime,/sys/info/sw_rev&fmt=obj` reads the named variables.
- `match=/sys/devices/&fmt=obj` reads every variable whose name contains the
  prefix.

With `fmt=obj` the answer is one flat JSON object of name to value, and every
value is a string, numbers included. An unknown name anywhere in a `name=`
list, or a `match=` that finds nothing, fails the whole request with 400.
Unset variables read as `TBD`, `N/A` or an empty string.

**Writes.** `POST /vars` with `set=/sys/telemetryws/enable=1&fmt=obj` sets a
variable and answers with it.

## Gateway variables

Read together every 30 seconds:

| Variable | Meaning |
|---|---|
| `/sys/info/cpu_usage`, `ram_usage`, `flash_usage` | percent |
| `/sys/info/uptime` | seconds since boot |
| `/sys/info/model` | `PVS5` or `PVS6` |
| `/sys/info/hwrev`, `sw_rev` | hardware revision, software version |
| `/sys/info/sys_type` | `PV-only` or `Storage`; varserver's default is `Unknown` |
| `/sys/livedata/backupTimeRemaining` | minutes of battery backup |
| `/sys/livedata/midstate` | microgrid interconnect device state |

Read every 5 minutes, and whenever `sys_type` changes, since they change
rarely and which exist depends on the model and system type:

| Variable | Meaning | Only on |
|---|---|---|
| `/sys/info/active_interface`, `boardtype`, `fwrev` | network uplink, board, firmware revision | |
| `/sys/toggle_cell/broadband_connected`, `cell_connected`, `low_data_mode` | 0 or 1 | |
| `/net/<interface>/state` | link state, for `cell`, `plc`, `sta0`, `wan` on a PVS5 and `sta0`, `wan0`, `wan1`, `wwan0` on a PVS6 | |
| `/sys/pvs/flashwear_type_b` | eMMC lifetime used, in hex tenths (`0x02` is 20%) | PVS6 |
| `/sys/pvs/usb_erase_count` | USB drive erase count | PVS6 |
| `/ess/config/dcm/control_param/min_customer_soc` | backup reserve, 0-1 | PVS6, Storage |
| `/ess/config/dcm/mode_param/control_mode` | configured battery mode | PVS6, Storage |

## Devices

One `match=/sys/devices/` returns every device as
`/sys/devices/<kind>/<index>/<param>`, with kinds `inverter`, `meter`, `ess`
and `transfer_switch`. Each device has a serial number `sn`, a model
`prodMdlNm`, and `msmtEps`, the time of its measurement (e.g.
`2026-10-14T08:25:00Z`). The PVS measures every device at once, every 5
minutes, so all devices carry the same `msmtEps` and reading more often
returns the same measurement. The exporter reads devices every minute so a new
measurement shows within one. Until the PVS lists a device, for example right
after a reboot, the match answers 400.

Power is in kW, apparent power in kVA, reactive power in kvar, energy in
kWh, and the rest in V, A, Hz, °C or 0-1. Per-leg values end in `1` or `2`,
e.g. `v1nV` and `v2nV`. The params the exporter reads:

| Kind | Params |
|---|---|
| `inverter` | `p3phsumKw`, `ltea3phsumKwh`, `tHtsnkDegc`, `vln3phavgV`, `i3phsumA`, `freqHz`; DC side `pMppt1Kw`, `vMppt1V`, `iMppt1A` |
| `meter` | `p3phsumKw`, `s3phsumKva`, `q3phsumKvar`, `totPfRto`, `vln3phavgV`, `v12V`, `i3phsumA`, `freqHz`, `ctSclFctr`, `posLtea3phsumKwh` (import), `negLtea3phsumKwh` (export), `netLtea3phsumKwh`; per leg `p1Kw`, `i1A`, `v1nV` |
| `ess` | `p3phsumKw`, `vBattV`, `min`/`maxTBattCellDegc`, `min`/`maxVBattCellV`, `socVal`, `customerSocVal`, `sohVal`, `chrgLimitPmaxKw`, `dischrgLimPmaxKw`, `tInvtrDegc`, `negLtea3phsumKwh` (charged), `posLtea3phsumKwh` (discharged), `opMode`; per leg `v1nV` |
| `transfer_switch` | `vSpplyV`, `tDegc`, `midStEnum`, `pvd1StEnum`; per leg `v1nGridV`, `v1nV` |

## Live data

`ws://<host>:9002` serves the live data with no authentication, once
`/sys/telemetryws/enable` is set to 1. A PVS5 has no such variable and
answers 400. The PVS sends one text frame a second, and over a 6-hour test no
pings, fragmented frames or close frames:

```json
{"notification": "power", "params": {"time": 1790000000, "soc": 0.5, "pv_p": 2.5, "net_p": -1.2, "site_load_p": 0.8, "ess_p": -0.5, "pv_en": 12000.5, "net_en": 3000.25, "site_load_en": 9500.75, "ess_en": -500.0}}
```

`time` is Unix seconds, `soc` is 0-1, the `_p` params are kW and the `_en`
params kWh. `net` is positive while importing and `ess` positive while
discharging. `backupTimeRemaining` and `midstate` are not in the frames; they
are read with the gateway variables.
