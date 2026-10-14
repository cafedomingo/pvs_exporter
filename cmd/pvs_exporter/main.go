// Command pvs_exporter exports a SunStrong PVS's data as Prometheus metrics.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/cafedomingo/pvs_exporter/internal/exporter"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := run(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfg, err := exporter.ConfigFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return err
	}
	slog.Info("serving /metrics", "address", ln.Addr().String(), "pvs", cfg.PVSHost, "push", cfg.PushURL != "")
	return exporter.Run(ctx, cfg, ln)
}
