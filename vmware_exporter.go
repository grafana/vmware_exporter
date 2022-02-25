package main

import (
	"context"
	"flag"
	"net/http"
	"os"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/grafana/vmware_exporter/vsphere"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

type Exporter struct {
	cfg    *Config
	logger log.Logger
	server *http.Server
}

func New(logger log.Logger, cfg *Config) (*Exporter, error) {
	e := &Exporter{
		cfg: cfg,
	}
	if logger == nil {
		logger = log.NewNopLogger()
	}
	e.logger = logger

	registry := prometheus.NewPedanticRegistry()

	vsphereCollector, err := vsphere.NewVSphereCollector(
		log.With(logger, "vsphere", "vsphere"),
		newClient(),
	)
	if err != nil {
		return nil, err
	}
	registry.MustRegister(vsphereCollector)

	goCollector := collectors.NewGoCollector()
	registry.MustRegister(goCollector)
	buildInfoCollector := collectors.NewBuildInfoCollector()
	registry.MustRegister(buildInfoCollector)

	// create http server
	topMux := http.NewServeMux()
	topMux.Handle(cfg.TelemetryPath, newHandler(log.With(logger, "component", "handler"), registry))
	e.server = &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: topMux,
	}
	return e, nil
}

func (e *Exporter) Start() error {
	level.Debug(e.logger).Log("msg", "starting the server")
	defer level.Debug(e.logger).Log("msg", "server stopped")
	return web.ListenAndServe(e.server, e.cfg.TLSConfigPath, log.With(e.logger, "component", "web"))
}

type Config struct {
	ListenAddr    string
	TelemetryPath string
	TLSConfigPath string
}

var DefaultConfig = &Config{
	ListenAddr:    ":9237",
	TelemetryPath: "/metrics",
	TLSConfigPath: "",
}

func (c *Config) RegisterFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.ListenAddr, "web.listen-address", DefaultConfig.ListenAddr,
		"Address on which to expose metrics and web interface.")
	fs.StringVar(&c.TelemetryPath, "web.telemetry-path", DefaultConfig.TelemetryPath,
		"Path under which to expose metrics.")
	fs.StringVar(&c.TLSConfigPath, "web.config", DefaultConfig.TLSConfigPath,
		"Path to config yaml file that can enable TLS or authentication.")
}

type handler struct {
	logger      log.Logger
	promHandler http.Handler
}

func newHandler(logger log.Logger, registry *prometheus.Registry) *handler {
	promHandler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorLog:            nil,
		ErrorHandling:       promhttp.PanicOnError,
		Registry:            nil,
		DisableCompression:  false,
		MaxRequestsInFlight: 0,
		Timeout:             0,
		EnableOpenMetrics:   false,
	})
	return &handler{
		logger:      logger,
		promHandler: promHandler,
	}
}

// ServeHTTP implements http.Handler.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	level.Debug(h.logger).Log("msg", "serving request")
	h.promHandler.ServeHTTP(w, r)
}

func newClient() *vim25.Client {
	u, err := soap.ParseURL(os.Getenv("GOVC_URL"))
	if err != nil {
		panic(err)
	}

	// Share govc's session cache
	s := &cache.Session{
		URL:      u,
		Insecure: true,
	}

	c := new(vim25.Client)
	err = s.Login(context.TODO(), c, nil)
	if err != nil {
		panic(err)
	}

	return c
}

func main() {
	cfg := &Config{}
	cfg.RegisterFlags(flag.CommandLine)
	flag.Parse()

	var logger log.Logger
	logger = log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
	logger = level.NewFilter(logger, level.AllowDebug())

	e, err := New(log.With(logger, "component", "exporter"), cfg)
	if err != nil {
		level.Error(logger).Log("msg", "could not create new exporter", "err", err)
		os.Exit(1)
	}
	if err := e.Start(); err != nil {
		level.Error(logger).Log("msg", "error running exporter", "err", err)
		os.Exit(2)
	}
}
