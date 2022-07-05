package vsphere

import (
	"bufio"
	"crypto/tls"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

func createSim(folders int) (*simulator.Model, *simulator.Server, error) {
	m := simulator.VPX()

	m.Folder = folders
	m.Datacenter = 2

	err := m.Create()
	if err != nil {
		return nil, nil, err
	}

	m.Service.TLS = new(tls.Config)

	s := m.Service.NewServer()
	return m, s, nil
}

func TestExporter(t *testing.T) {
	m, s, err := createSim(0)
	defer m.Remove()
	defer s.Close()
	if err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		TelemetryPath:           "/metrics",
		VSphereURL:              s.URL,
		TLSConfigPath:           "",
		ChunkSize:               5,
		ObjectDiscoveryInterval: 0,
		EnableExporterMetrics:   false,
	}
	e, err := NewExporter(nil, cfg)
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
		log.Fatal(err)
	}

	f, err := os.Open("test_metrics.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// Check if the line is in the response body
		if !strings.Contains(allMetrics, scanner.Text()) {
			t.Errorf("Expected metrics to contain '%s'", scanner.Text())
		}
	}
}
