package vsphere

import (
	"context"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/exporter-toolkit/web"
	"net/http"
)

type Exporter struct {
	cfg    *Config
	logger log.Logger
	server *http.Server
}

func NewExporter(logger log.Logger, cfg *Config) (*Exporter, error) {
	ctx := context.Background()
	x := &Exporter{
		cfg: cfg,
	}
	if logger == nil {
		logger = log.NewNopLogger()
	}
	x.logger = logger

	registry := prometheus.NewPedanticRegistry()

	e, err := newEndpoint(defaultVSphere, cfg.VSphereURL, logger)
	if err != nil {
		return nil, err
	}
	vsphereCollector, err := newVSphereCollector(
		ctx,
		log.With(logger, "vsphere", "vsphere"),
		e,
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
	x.server = &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: topMux,
	}
	return x, nil
}

func (e *Exporter) Start() error {
	level.Debug(e.logger).Log("msg", "starting the server")
	defer level.Debug(e.logger).Log("msg", "server stopped")
	return web.ListenAndServe(e.server, e.cfg.TLSConfigPath, log.With(e.logger, "component", "web"))
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
