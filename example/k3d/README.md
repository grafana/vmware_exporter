# Local Environment

This example documents how to get up and running locally with the vmware_exporter for development purposes. In this setup, the exporter collects performance counters from vcsim and forwards them to cortex in k3d.

## Command Quick Reference

| Program | Command |
| ------- | ------- |
| vcsim | ~/go/bin/vcsim |
| vmware_exporter |  go run vmware_exporter.go -vsphere.url https://user:pass@127.0.0.1:8989/sdk |
| Prometheus Agent | prometheus --enable-feature=agent,extra-scrape-metrics --web.enable-admin-api --config.file=./prometheus.yml |
| k3d | k3d cluster start lgtm |

## k3d with cortex / grafana

### Create k3d cluster

```bash
./scripts/k3d-cluster
kubectl cluster-info
```

Make note of the kubernetes control plane address.

### Stand-up Grafana, Cortex

Replace the address passed to --server with your control plane address.

```bash
tk env set environments/default --server=https://0.0.0.0:54974
tk apply environments/default
```

If everything worked correctly, Grafana and Cortex should be up and running.
Grafana: http://grafana.k3d.localhost:50080/
Cortex: http://cortex.k3d.localhost:50080/

## vcsim

The vcsim program is a vCenter and ESXi API based simulator. Follow the vcsim installation instructions [here](https://github.com/vmware/govmomi/tree/master/vcsim).

```bash
~/go/bin/vcsim
```

## Prometheus Agent

You'll need Prometheus installed for this step. 

```bash
prometheus --enable-feature=agent,extra-scrape-metrics --web.enable-admin-api --config.file=./prometheus.yml
```

Check the configured targets:
http://localhost:9090/targets

## vmware_exporter

Now we're ready to run the exporter:

```bash
go run vmware_exporter.go -vsphere.url https://user:pass@127.0.0.1:8989/sdk
```

You should see vSphere metrics gathered from vcsim here:
http://localhost:9237/metrics

