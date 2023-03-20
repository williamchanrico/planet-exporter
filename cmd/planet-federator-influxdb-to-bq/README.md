## Planet Federator Pre-processor from InfluxDB to BQ

A standard planet-exporter setup:

1. Planet Exporter (exposes detailed network L4 traffic data)
2. Prometheus (scrapes everything)
3. Planet Federator (aggregates Prometheus data to InfluxDB)
4. InfluxDB
5. **This tool aggregates further, from InfluxDB to BQ**

### Structure

How the tool and packages are structured:

1. The `cmd/planet-federator-influxdb-to-bq` package contain a standalone analysis logic, including a simple BQ backend implementation.
2. It uses only 1 other package, `federator/influxdb/query`, to access the InfluxDB data via compatible API version.
FYI we keep the `query` package in the `federator/influxdb` structure for easier upkeep, because it must use the same version of InfluxDB API that federator is using.

3. Cron mechanism is utilized to periodically trigger 2 analysis implementations: (1) hourly traffic data query and (2) daily dependency data query.

The query happens in a straightforward manner:

1. It queries InfluxDB using v1 API (using InfluxQL and not Flux).
2. It processes the data within its own internal package.
3. It stores the results in 2 BigQuery Tables (Traffic & Dependency data).

### Analysis 01: Traffic Data (Hourly)

Service-to-service traffic bandwidth in bits (1h min, max, & avg).

| Field                          | Example Value                  |
|--------------------------------|--------------------------------|
| inventory_date                 | 2023-03-20 04:22:40.005178 UTC |
|  traffic_direction             | ingress                        |
|  local_hostgroup               | myservice                      |
|  local_hostgroup_address       | myservice.service.consul       |
|  remote_hostgroup              | other-service                  |
|  remote_hostgroup_address      | other-service.service.consul   |
|  traffic_bandwidth_bits_min_1h |                           2304 |
|  traffic_bandwidth_bits_max_1h |                           9469 |
|  traffic_bandwidth_bits_avg_1h |                           6823 |

### Analysis 02: Dependency Data (Daily)

Service-to-service dependency.

| Field                          | Example Value                  |
|--------------------------------|--------------------------------|
| inventory_date                 | 2023-03-20 12:51:00.000275 UTC |
|  dependency_direction          | upstream                       |
|  protocol                      | tcp                            |
|  local_hostgroup_process_name  | myservice-process-name         |
|  local_hostgroup               | myservice                      |
|  local_hostgroup_address       | myservice.service.consul       |
|  local_hostgroup_address_port  | 80                             |
|  remote_hostgroup              | other-service                  |
|  remote_hostgroup_address      | other-service.service.consul   |
|  remote_hostgroup_address_port | 80                             |

