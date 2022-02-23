package vsphere

import (
	"context"
	"fmt"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"
	"strings"
)

type vsphereCollector struct {
	logger log.Logger
	client *vim25.Client
}

func (c *vsphereCollector) Describe(descs chan<- *prometheus.Desc) {
	level.Debug(c.logger).Log("msg", "describe")
}

func (c *vsphereCollector) Collect(metrics chan<- prometheus.Metric) {
	ctx := context.TODO()
	viewManager := view.NewManager(c.client)
	perfManager := performance.NewManager(c.client)

	v, err := viewManager.CreateContainerView(ctx, c.client.ServiceContent.RootFolder, nil, true)
	if err != nil {
		return
	}

	defer v.Destroy(ctx)

	refs, err := v.Find(ctx, []string{
		"VirtualMachine",
		"ClusterComputeResource",
		"Datacenter",
		"Datastore",
		"HostSystem",
		"ResourcePool",
	}, nil)
	if err != nil {
		level.Debug(c.logger).Log("msg", "error in find", "err", err)
		return
	}

	// Retrieve counters name list
	counters, err := perfManager.CounterInfoByName(ctx)
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
	sample, err := perfManager.SampleByName(ctx, spec, names, refs)
	if err != nil {
		level.Debug(c.logger).Log("msg", "error getting sample by name", "err", err)
		return
	}

	result, err := perfManager.ToMetricSeries(ctx, sample)
	if err != nil {
		level.Debug(c.logger).Log("err", err)
		return
	}

	// Read result
	for _, metric := range result {
		name := strings.Split(fmt.Sprintf("%s", metric.Entity), ":")[1]
		//collectMetric(metrics, &metric, "vm", name.Value)
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
				m, err := prometheus.NewConstMetric(desc, prometheus.UntypedValue, float64(v.Value[0]))
				if err != nil {
					level.Error(c.logger).Log("err", err)
					continue
				}
				metrics <- m

			}
		}
	}
}

func NewVSphereCollector(logger log.Logger, client *vim25.Client) (prometheus.Collector, error) {
	if logger == nil {
		logger = log.NewNopLogger()
	}
	return &vsphereCollector{
		logger: logger,
		client: client,
	}, nil
}
