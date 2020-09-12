## Planet Exporter

Simple discovery space-ship for your ~~infrastructure~~ planetary ecosystem across the universe.

Measure an environment's potential to maintain ~~services~~ life.

* The `task/*` packages are the crew that does expensive task behind the scene and prepare the data for `collector` package.
* The `collector` package calls one/more `task/*` packages if it needs expensive metrics instead of preparing them on-the-fly.

#### Collector Tasks

##### Inventory

Query inventory data to map IP into `hostgroup` (an identifier based on ansible convention) and `domain`.

URL should contain an array of json objects like:

```json
[
  {
    "ip_address": "10.1.2.3",
    "domain": "xyz.service.consul",
    "hostgroup": "xyz"
  },
  {
    "ip_address": "10.2.3.4",
    "domain": "debugapp.service.consul",
    "hostgroup": "debugapp"
  }
]
```

#### Socketstat

Query local connections socket similar to `ss` or `netstat` to build upstream and downstream dependency metrics.

```
# HELP planet_upstream Upstream dependency of this machine
# TYPE planet_upstream gauge
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="80",protocol="tcp",remote_address="xyz.service.consul",remote_hostgroup="xyz"} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="8500",protocol="tcp",remote_address="xyz.service.consul",remote_hostgroup="xyz"} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="8300",protocol="tcp",remote_address="10.2.3.3",remote_hostgroup="consul-server"} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="8300",protocol="tcp",remote_address="10.2.3.4",remote_hostgroup="consul-server"} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="3128",protocol="tcp",remote_address="100.100.98.18",remote_hostgroup=""} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="443",protocol="tcp",remote_address="35.158.25.125",remote_hostgroup=""} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="443",protocol="tcp",remote_address="52.219.32.222",remote_hostgroup=""} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="80",protocol="tcp",remote_address="100.100.103.57",remote_hostgroup=""} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="80",protocol="tcp",remote_address="100.100.30.26",remote_hostgroup=""} 1
# HELP planet_downstream Downstream dependency of this machine
# TYPE planet_downstream gauge
planet_downstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="9100",protocol="tcp",remote_address="prometheus.service.consul",remote_hostgroup="prometheus"} 1
planet_downstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="19100",protocol="tcp",remote_address="prometheus.service.consul",remote_hostgroup="prometheus"} 1
planet_downstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="19100",protocol="tcp",remote_address="192.168.1.2",remote_hostgroup=""} 1
planet_downstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="22",protocol="tcp",remote_address="192.168.1.2",remote_hostgroup=""} 1
```

##### Darkstat

[Darkstat](https://unix4lyfe.org/darkstat/) captures network traffic, calculates statistics about usage, and serves reports over HTTP.

Data from darkstat can be leveraged for network dependencies capture.
That means we'll have to install darkstat along with planet-exporter.

Example metrics:

```
# HELP planet_traffic_bytes_total Total network traffic with peers
# TYPE planet_traffic_bytes_total gauge
planet_traffic_bytes_total{direction="egress",remote_domain="xyz.service.consul",remote_hostgroup="xyz",remote_ip="10.1.2.3"} 2005
planet_traffic_bytes_total{direction="egress",remote_domain="debugapp.service.consul",remote_hostgroup="debugapp",remote_ip="10.2.3.4"} 150474
planet_traffic_bytes_total{direction="ingress",remote_domain="xyz.service.consul",remote_hostgroup="xyz",remote_ip="10.1.2.3"} 2525
planet_traffic_bytes_total{direction="ingress",remote_domain="debugapp.service.consul",remote_hostgroup="debugapp",remote_ip="10.2.3.4"} 1.26014316e+08
```

## Contributing

Please read [CONTRIBUTING.md](./CONTRIBUTING.md) for details on our code of conduct, and the process for submitting pull requests to us.
