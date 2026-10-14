# Packages the binaries the release workflow cross-compiled into dist/, so no
# platform needs emulation.
FROM gcr.io/distroless/static-debian13:nonroot
ARG TARGETARCH
ARG TARGETVARIANT
COPY dist/pvs_exporter-linux-${TARGETARCH}${TARGETVARIANT} /pvs_exporter
EXPOSE 9711
ENTRYPOINT ["/pvs_exporter"]
