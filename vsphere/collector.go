package vsphere

import (
	"context"
	"fmt"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/vmware/govmomi/vim25/types"
	"strings"
)

type vsphereCollector struct {
	logger   log.Logger
	endpoint *endpoint
}

func (c *vsphereCollector) Describe(descs chan<- *prometheus.Desc) {
	level.Debug(c.logger).Log("msg", "describe")
}

func (c *vsphereCollector) Collect(metrics chan<- prometheus.Metric) {
	ctx := context.Background()

	myClient, err := c.endpoint.clientFactory.GetClient(ctx)
	if err != nil {
		level.Debug(c.logger).Log("msg", "error getting client", "err", err)
		return
	}

	if c.endpoint.cfg.ObjectDiscoveryInterval == 0 {
		err = c.endpoint.discover(ctx)
		if err != nil && err != context.Canceled {
			level.Error(c.logger).Log("msg", "discovery error", "host", c.endpoint.url.Host, "err", err.Error())
			return
		}
	}

	var refs []types.ManagedObjectReference
	for _, r := range c.endpoint.resourceKinds {
		for _, o := range r.objects {
			refs = append(refs, o.ref)
		}
	}

	// Retrieve counters name list
	counters, err := myClient.counterInfoByName(ctx)
	if err != nil {
		level.Debug(c.logger).Log("msg", "error getting counters", "err", err)
		return
	}

	var names []string
	for name := range counters {
		names = append(names, name)
	}

	// Create PerfQuerySpec
	spec := types.PerfQuerySpec{
		MaxSample:  1,
		MetricId:   []types.PerfMetricId{{Instance: "*"}},
		IntervalId: 20,
	}

	// Query metrics
	sample, err := myClient.Perf.SampleByName(ctx, spec, names, refs)
	if err != nil {
		level.Debug(c.logger).Log("msg", "error getting sample by name", "err", err)
		return
	}

	result, err := myClient.Perf.ToMetricSeries(ctx, sample)
	if err != nil {
		level.Debug(c.logger).Log("err", err)
		return
	}

	// Read result
	for _, metric := range result {
		name := strings.Split(fmt.Sprintf("%s", metric.Entity), ":")[1]
		for _, v := range metric.Value {
			counter := counters[v.Name]
			units := counter.UnitInfo.GetElementDescription().Label

			instance := v.Instance
			if instance == "" {
				instance = "-"
			}

			if len(v.Value) != 0 {

				// get fqName
				fqName := fmt.Sprintf("vsphere_%s_%s", metric.Entity.Type, strings.ReplaceAll(v.Name, ".", "_"))

				// create desc
				constLabels := make(prometheus.Labels)
				constLabels["name"] = name
				desc := prometheus.NewDesc(
					fqName, fmt.Sprintf("metric: %s units: %s", v.Name, units),
					nil,
					constLabels)

				// send metric
				m, err := prometheus.NewConstMetric(desc, prometheus.GaugeValue, float64(v.Value[0]))
				if err != nil {
					level.Error(c.logger).Log("err", err)
					continue
				}
				metrics <- m

			}
		}
	}
}

func newVSphereCollector(ctx context.Context, logger log.Logger, e *endpoint) (prometheus.Collector, error) {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	err := e.init(ctx)
	if err != nil {
		return nil, err
	}

	return &vsphereCollector{
		logger:   logger,
		endpoint: e,
	}, nil
}
