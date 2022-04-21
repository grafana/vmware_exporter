package main

import (
	"flag"
	"os"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/vmware_exporter/vsphere"
)

func main() {
	cfg := &vsphere.Config{}
	cfg.RegisterFlags(flag.CommandLine)
	flag.Parse()

	var logger log.Logger
	logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
	logger = level.NewFilter(logger, level.AllowDebug())

	e, err := vsphere.NewExporter(logger, cfg)
	if err != nil {
		level.Error(logger).Log("msg", "could not create new exporter", "err", err)
		os.Exit(1)
	}
	if err := e.Start(); err != nil {
		level.Error(logger).Log("msg", "error running exporter", "err", err)
		os.Exit(2)
	}
}
