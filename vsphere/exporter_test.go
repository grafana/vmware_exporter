package vsphere

import (
	"bufio"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/vmware/govmomi/simulator"
)

func createSim(folders int) (*simulator.Model, *simulator.Server, error) {
	m := simulator.VPX()

	m.Folder = folders
	m.Datacenter = 2
	m.Cluster = 2
	m.Host = 4
	m.Machine = 8

	err := m.Create()
	if err != nil {
		return nil, nil, err
	}

	m.Service.TLS = new(tls.Config)

	s := m.Service.NewServer()
	return m, s, nil
}

type testLogger struct {
	T *testing.T
}

func (l testLogger) Write(p []byte) (n int, err error) {
	l.T.Logf(string(p))
	return len(p), nil
}

func TestExporter(t *testing.T) {
	var logger log.Logger
	logger = log.NewLogfmtLogger(log.NewSyncWriter(testLogger{
		T: t,
	}))
	logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
	logger = level.NewFilter(logger, level.AllowDebug())

	m, s, err := createSim(0)
	defer m.Remove()
	defer s.Close()
	if err != nil {
		t.Fatal(err)
	}

	type args struct {
		logger log.Logger
		cfg    *Config
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test exporter",
			args: args{
				logger: logger,
				cfg: &Config{
					TelemetryPath:           "/metrics",
					VSphereURL:              s.URL,
					TLSConfigPath:           "",
					ChunkSize:               256,
					ObjectDiscoveryInterval: 0,
					EnableExporterMetrics:   false,
				},
			},
		},
		{
			name: "test exporter - non-zero discovery interval",
			args: args{
				logger: logger,
				cfg: &Config{
					TelemetryPath:           "/metrics",
					VSphereURL:              s.URL,
					TLSConfigPath:           "",
					ChunkSize:               256,
					ObjectDiscoveryInterval: 60 * time.Second,
					EnableExporterMetrics:   false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := NewExporter(logger, tt.args.cfg)
			if err != nil {
				t.Fatal(err)
			}

			req, err := http.NewRequest("GET", "/metrics", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			e.server.Handler.ServeHTTP(rr, req)

			if status := rr.Code; status != http.StatusOK {
				t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
			}

			allMetrics := rr.Body.String()
			if err != nil {
				level.Error(logger).Log("err", err)
				t.Fatal(err)
			}

			f, err := os.Open("test_metrics.txt")
			if err != nil {
				t.Fatal(err)
			}

			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				// Check if the line is in the response body
				if !strings.Contains(allMetrics, scanner.Text()) {
					t.Errorf("Expected metrics to contain '%s'", scanner.Text())
				}
			}
			_ = f.Close()
		})
	}
}
