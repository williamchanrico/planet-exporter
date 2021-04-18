<h1 align="center">Planet Exporter</h1>

<div align="center">
  :house_with_garden:
</div>
<div align="center">
  <strong>Know your dependencies!</strong>
</div>
<div align="center">
  An <code>experimental</code> code to support my other project.
</div>

<br />

<div align="center">
  <!-- Stability -->
  <a href="https://nodejs.org/api/documentation.html#documentation_stability_index">
    <img src="https://img.shields.io/badge/stability-experimental-orange.svg?style=flat-square"
      alt="API stability" />
  </a>
  <!-- Relese -->
  <a href="https://github.com/williamchanrico/planet-exporter/releases">
    <img src="https://img.shields.io/github/release/williamchanrico/planet-exporter.svg?style=flat-square""
      alt="Release" />
  </a>
  <!-- Apache License -->
  <a href="https://opensource.org/licenses/Apache-2.0"><img
	src="https://img.shields.io/badge/License-Apache%202.0-blue.svg"
	border="0"
	alt="Apache-2.0 Licence"
	title="Apache-2.0 Licence">
  </a>
  <!-- Open Source Love -->
  <a href="#"><img
	src="https://badges.frapsoft.com/os/v1/open-source.svg?v=103"
	border="0"
	alt="Open Source Love"
	title="Open Source Love">
  </a>
</div>

## Introduction

The goal is to determine every servers' dependencies (upstream/downstream) along with bandwidth required for those dependencies.

Simple discovery space-ship for your ~~infrastructure~~ planetary ecosystem across the universe.

Measure an environment's potential to maintain ~~services~~ life.

### Packages Structure

* The `task/*` packages are the crew that does expensive task behind the scene and prepare the data for `collector` package.
* The `collector` package calls one/more `task/*` packages if it needs expensive metrics instead of preparing them on-the-fly.

### Installation

Grab a pre-built binary for your OS from the [Releases](https://github.com/williamchanrico/planet-exporter/releases/latest) page.

### Configuration

Flags:

There's no required flags. It is configured with usable defaults.

```
Usage of planet-exporter:
  -listen-address string
    	Address to which exporter will bind its HTTP interface (default "0.0.0.0:19100")
  -log-disable-colors
    	Disable colors on logger
  -log-disable-timestamp
    	Disable timestamp on logger
  -log-level string
    	Log level (default "info")
  -task-darkstat-addr string
    	Darkstat target address
  -task-darkstat-enabled
    	Enable darkstat collector task
  -task-interval string
    	Interval between collection of expensive data into memory (default "7s")
  -task-inventory-addr string
    	Darkstat target address
  -task-inventory-enabled
    	Enable inventory collector task
  -task-socketstat-enabled
    	Enable socketstat collector task (default true)
  -version
    	Show version and exit
```

Running with minimum collector tasks (just the socketstat)

```
# planet-exporter
```

Running with inventory and darkstat (installed separately rev >= [e7e6652](https://www.unix4lyfe.org/gitweb/darkstat/commit/e7e6652113099e33930ab0f39630bf280e38f769))

```
# planet-exporter \
	-task-inventory-enabled \
	-task-inventory-addr http://link-to-your.net/inventory_hosts.json \
	-task-darkstat-enabled \
	-task-darkstat-addr http://localhost:51666/metrics
```

### Collector Tasks

#### Inventory

Query inventory data to map IP into `hostgroup` (an identifier based on ansible convention) and `domain`.

Without this task enabled, those hostgroup and domain fields will be left empty.

The flag `--task-inventory-addr` should contain an http url to an array of json objects:

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
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="80",process_name="debugapp",protocol="tcp",remote_address="xyz.service.consul",remote_hostgroup="xyz"} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="8500",process_name="consul-template",protocol="tcp",remote_address="127.0.0.1",remote_hostgroup="localhost"} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="8300",process_name="consul",protocol="tcp",remote_address="10.2.3.3",remote_hostgroup="consul-server"} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="8300",process_name="consul",protocol="tcp",remote_address="10.2.3.4",remote_hostgroup="consul-server"} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="3128",process_name="",protocol="tcp",remote_address="100.100.98.18",remote_hostgroup=""} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="443",process_name="",protocol="tcp",remote_address="35.158.25.125",remote_hostgroup=""} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="443",process_name="",protocol="tcp",remote_address="52.219.32.222",remote_hostgroup=""} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="80",process_name="cloudmetrics",protocol="tcp",remote_address="100.100.103.57",remote_hostgroup=""} 1
planet_upstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="80",process_name="cloudmetrics",protocol="tcp",remote_address="100.100.30.26",remote_hostgroup=""} 1
# HELP planet_downstream Downstream dependency of this machine

# TYPE planet_downstream gauge
planet_downstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="9100",process_name="node_exporter",protocol="tcp",remote_address="prometheus.service.consul",remote_hostgroup="prometheus"} 1
planet_downstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="19100",process_name="planet-exporter",protocol="tcp",remote_address="prometheus.service.consul",remote_hostgroup="prometheus"} 1
planet_downstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="19100",process_name="planet-exporter",protocol="tcp",remote_address="192.168.1.2",remote_hostgroup=""} 1
planet_downstream{local_address="debugapp.service.consul",local_hostgroup="debugapp",port="22",process_name="sshd",protocol="tcp",remote_address="192.168.1.2",remote_hostgroup=""} 1

# HELP planet_server_process Server process that are listening on network interfaces
# TYPE planet_server_process gauge
planet_server_process{bind="0.0.0.0:111",port="111",process_name="rpcbind"} 1
planet_server_process{bind="0.0.0.0:19100",port="19100",process_name="planet-exporter"} 1
planet_server_process{bind="0.0.0.0:22",port="22",process_name="sshd"} 1
planet_server_process{bind="0.0.0.0:25",port="25",process_name="master"} 1
planet_server_process{bind="0.0.0.0:5666",port="5666",process_name="nrpe"} 1
planet_server_process{bind="0.0.0.0:80",port="80",process_name="nginx"} 1
planet_server_process{bind="127.0.0.1:53",port="53",process_name="consul"} 1
planet_server_process{bind="127.0.0.1:8500",port="8500",process_name="consul"} 1
planet_server_process{bind="0.0.0.0:51666",port="51666",process_name="darkstat"} 1
planet_server_process{bind=":::111",port="111",process_name="rpcbind"} 1
planet_server_process{bind=":::25",port="25",process_name="master"} 1
planet_server_process{bind=":::50051",port="50051",process_name="socketmaster"} 1
planet_server_process{bind=":::5666",port="5666",process_name="nrpe"} 1
planet_server_process{bind=":::8301",port="8301",process_name="consul"} 1
planet_server_process{bind=":::9000",port="9000",process_name="socketmaster"} 1
planet_server_process{bind=":::9100",port="9100",process_name="node_exporter"} 1
planet_server_process{bind=":::9256",port="9256",process_name="process_exporte"} 1
```

#### Darkstat

[Darkstat](https://unix4lyfe.org/darkstat/) captures network traffic, calculates statistics about usage, and serves reports over HTTP.

Though there's no port detection from darkstat to determine remote/local port for each traffic direction, the bandwidth information can still be useful.

NOTE: this means we'll have to install darkstat along with planet-exporter.

Example parsed metrics from darkstat when enabled (plus inventory task for `remote_domain` and `remote_hostgroup`):

```
# HELP planet_traffic_bytes_total Total network traffic with peers
# TYPE planet_traffic_bytes_total gauge
planet_traffic_bytes_total{direction="egress",remote_domain="xyz.service.consul",remote_hostgroup="xyz",remote_ip="10.1.2.3"} 2005
planet_traffic_bytes_total{direction="egress",remote_domain="debugapp.service.consul",remote_hostgroup="debugapp",remote_ip="10.2.3.4"} 150474
planet_traffic_bytes_total{direction="ingress",remote_domain="xyz.service.consul",remote_hostgroup="xyz",remote_ip="10.1.2.3"} 2525
planet_traffic_bytes_total{direction="ingress",remote_domain="debugapp.service.consul",remote_hostgroup="debugapp",remote_ip="10.2.3.4"} 1.26014316e+08
```

## Exporter Cost

Planet exporter will consume CPU and Memory in proportion to the number
of opened network file descriptors (opened sockets).

## Additional Binaries

### Planet Federator

Since planet-exporter stores raw data in Prometheus, dashboard queries on those data can get expensive.
A tested traffic bandwidth query for a crowded service with ~300 upstreams/downstreams took about `9s` to return 1h data range.
It gets longer when querying 12h or days range of data.

Planet Federator runs a Cron that queries Planet Exporter's traffic bandwidth data from Prometheus, pre-process, and
stores them in a time-series database for clean and efficient dashboard queries.

Latest tested query on pre-processed data from InfluxDB for a crowded service that took `8.494s`, now takes `2.259s`.

TSDB supports:
- [x] InfluxDB
- [ ] Prometheus (if InfluxDB turns out to be a bad choice)
- [ ] BigQuery

#### Example InfluQL

```sql
SELECT
	SUM("bandwidth_bps")
FROM
	"ingress"
WHERE
	("service" = '$service') AND $timeFilter
GROUP BY
	time($__interval), "service", "remote_service", "remote_address"
```

```sh
$ planet-federator \
	-prometheus-addr "http://127.0.0.1:9090" \
	-influxdb-addr "http://127.0.0.1:8086" \
	-influxdb-bucket "mothership" # Works as database name if you're using InfluxDB v1.8 and earlier
```

## Used Go Version

```
$ go version
go version go1.15 linux/amd64
```

> Older Go version should work fine.

## Contributing

Pull requests for new features, bug fixes, and suggestions are welcome!

## License

[Apache License 2.0](https://github.com/williamchanrico/planet-exporter/blob/master/LICENSE)
