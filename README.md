# vmware_exporter

Note: This project is still in the early stages of development and not considered production ready. Very little testing
against live environments has been done; defects are to be expected at this stage. With that said, any feedback is
greatly appreciated.

## Collect vSphere Performance Metrics
The vSphere collector connects to a vCenter sdk endpoint and discovers managed objects in the datacenter inventory.
Resource discovery will occur per scrape by default; however, it can also be configured to run in the background on an
interval by setting the discovery interval command line flag. Currently, most of the object discovery code is ported
from the telegraf vSphere plugin.

For all resources discovered, the collector will attempt to gather the latest sample of aggregated instance data
from the vSphere performance manager and expose them on the telemetry path (default /metrics).

## Usage
```
Usage of ./vmware_exporter:
  -vsphere.discovery-interval duration
        Object discovery duration interval. Discovery will occur per scrape if set to 0.
  -vsphere.mo-chunk-size int
        Managed object reference chunk size to use when fetching from vSphere. (default 5)
  -vsphere.url value
        vSphere SDK URL.
  -web.config string
        Path to config yaml file that can enable TLS or authentication.
  -web.listen-address string
        Address on which to expose metrics and web interface. (default ":9237")
  -web.telemetry-path string
        Path under which to expose metrics. (default "/metrics")
```

### Example
```
./vmware_exporter -vsphere.url http://user:pass@127.0.0.1:8989/sdk -vsphere.mo-chunk-size 10
```
