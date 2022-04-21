package vsphere

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/vmware/govmomi/vim25/types"
	"golang.org/x/sync/semaphore"
)

type vsphereCollector struct {
	logger   log.Logger
	endpoint *endpoint
}

func (c *vsphereCollector) Describe(chan<- *prometheus.Desc) {
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
		err := c.endpoint.discover(ctx)
		if err != nil && err != context.Canceled {
			level.Error(c.logger).Log("msg", "discovery error", "host", c.endpoint.url.Host, "err", err.Error())
			return
		}
	}

	now, err := myClient.getServerTime(ctx)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to get server time", "err", err.Error())
		return
	}

	sem := semaphore.NewWeighted(int64(c.endpoint.cfg.CollectConcurrency))

	counters, err := myClient.counterInfoByName(ctx)
	if err != nil {
		level.Debug(c.logger).Log("msg", "error getting counters", "err", err)
		return
	}

	var names []string
	for name := range counters {
		names = append(names, name)
	}

	var wg sync.WaitGroup
	for k, r := range c.endpoint.resourceKinds {
		if r.enabled {
			level.Debug(c.logger).Log("msg", "collecting metrics", "kind", k)
			wg.Add(1)
			go func(k string, res *resourceKind) {
				defer wg.Done()

				latest := res.latestSample
				if !latest.IsZero() {
					elapsed := now.Sub(latest).Seconds() + 5.0 // Allow 5 second jitter.
					//e.log.Debugf("Latest: %s, elapsed: %f, resource: %s", latest, elapsed, resourceType)
					if !res.realTime && elapsed < float64(res.sampling) {
						// No new data would be available. We're outta here!
						// TODO: log
						//e.log.Debugf("Sampling period for %s of %d has not elapsed on %s",
						//	resourceType, res.sampling, e.URL.Host)
						return
					}
				} else {
					latest = now.Add(time.Duration(-res.sampling) * time.Second)
				}

				var refs []types.ManagedObjectReference
				// TODO: telegraf caches these values
				start := latest.Add(time.Duration(-res.sampling) * time.Second * (time.Duration(c.endpoint.cfg.MetricLookback) - 1))
				for _, obj := range res.objects {
					refs = append(refs, obj.ref)
				}
				level.Debug(c.logger).Log("refs count", len(refs), "kind", k)

				spec := types.PerfQuerySpec{
					MaxSample:  1,
					MetricId:   []types.PerfMetricId{{Instance: "*"}},
					IntervalId: res.sampling,
					StartTime:  &start,
					EndTime:    &now,
				}

				// chunk refs
				var refChunks [][]types.ManagedObjectReference
				refsSize := len(refs)
				chunkSize := c.endpoint.cfg.RefChunkSize
				for i := 0; i < refsSize; i += chunkSize {
					end := i + chunkSize
					if end > refsSize {
						end = refsSize
					}
					refChunks = append(refChunks, refs[i:end])
				}

				var ccWg sync.WaitGroup
				for _, chunk := range refChunks {
					// use semaphore to execute chunks concurrently
					ccWg.Add(1)
					go func(s *semaphore.Weighted, chunk []types.ManagedObjectReference) {
						if err := s.Acquire(ctx, 1); err != nil {
							level.Error(c.logger).Log("msg", "error acquiring semaphore", "err", err)
							return
						}

						defer func() {
							ccWg.Done()
							sem.Release(1)
						}()

						sample, err := myClient.Perf.SampleByName(ctx, spec, names, chunk)
						if err != nil {
							level.Debug(c.logger).Log("msg", "error getting sample by name", "err", err)
							return
						}

						result, err := myClient.Perf.ToMetricSeries(ctx, sample)
						if err != nil {
							level.Debug(c.logger).Log("err", err)
							return
						}

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
									// TODO: should this be v.Value[0] or v.Value[len(v.Value)] to get the latest value for this metric?
									m, err := prometheus.NewConstMetric(desc, prometheus.GaugeValue, float64(v.Value[0]))
									if err != nil {
										level.Error(c.logger).Log("err", err)
										continue
									}
									metrics <- m
								}
							}
						}
					}(sem, chunk)
				}
				ccWg.Wait()

				latestSample := time.Time{}
				if !latestSample.IsZero() {
					res.latestSample = latestSample
				}
			}(k, r)
		}
	}

	wg.Wait()
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
