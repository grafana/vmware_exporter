package vsphere

import (
	"context"
	"net/http"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/exporter-toolkit/web"
)

// Exporter holds the data needed to run a vSphere exporter
type Exporter struct {
	cfg    *Config
	logger log.Logger
	server *http.Server
}

// NewExporter creates a new vSphere exporter from the given config
func NewExporter(logger log.Logger, cfg *Config) (*Exporter, error) {
	ctx := context.Background()
	x := &Exporter{
		cfg: cfg,
	}
	if logger == nil {
		logger = log.NewNopLogger()
	}
	x.logger = logger

	var mReg *prometheus.Registry
	if cfg.EnableMetaMetrics {
		mReg = prometheus.NewRegistry()
		goCollector := collectors.NewGoCollector()
		mReg.MustRegister(goCollector)
		buildInfoCollector := collectors.NewBuildInfoCollector()
		mReg.MustRegister(buildInfoCollector)
	}

	registry := prometheus.NewRegistry()
	defaultVSphere.ObjectDiscoveryInterval = cfg.ObjectDiscoveryInterval
	defaultVSphere.RefChunkSize = cfg.ChunkSize
	e, err := newEndpoint(defaultVSphere, cfg.VSphereURL, logger, mReg)
	if err != nil {
		return nil, err
	}
	vsphereCollector, err := newVSphereCollector(
		ctx,
		log.With(logger, "collector", "vsphere"),
		e,
	)
	if err != nil {
		return nil, err
	}
	registry.MustRegister(vsphereCollector)

	// create http server
	topMux := http.NewServeMux()
	topMux.Handle(cfg.TelemetryPath, newHandler(log.With(logger, "component", "handler"), registry))
	if cfg.EnableMetaMetrics {
		topMux.Handle("/-/metrics", newHandler(log.With(logger, "component", "meta_handler"), mReg))
	}
	x.server = &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: topMux,
	}
	return x, nil
}

// Start runs the exporter
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
