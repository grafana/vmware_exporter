// A Prometheus exporter for VMWare vSphere Performance Metrics
package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/grafana/vmware_exporter/vsphere"
)

func main() {
	cfg := &vsphere.Config{}
	cfg.RegisterFlags(flag.CommandLine)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	}))

	e, err := vsphere.NewExporter(logger, cfg)
	if err != nil {
		logger.Error("could not create new exporter", "err", err)
		os.Exit(1)
	}
	if err := e.Start(); err != nil {
		logger.Error("error running exporter", "err", err)
		os.Exit(2)
	}
}
