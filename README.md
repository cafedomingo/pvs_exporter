# pvs_exporter

Single-binary Prometheus exporter for the SunStrong/SunPower PVS5 and PVS6
gateways. It polls the PVS local API, serves `/metrics`, and pushes the
1-second live power stream to VictoriaMetrics.

MIT licensed.
