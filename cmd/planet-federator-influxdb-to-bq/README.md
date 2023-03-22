## Planet Federator Pre-processor from InfluxDB to BQ

In order to understand where this tool sits, let's look at how a standard planet-exporter setup may look like:

1. Planet Exporter (exposes detailed network L4 traffic data)
2. Prometheus (scrapes everything)
3. Planet Federator (aggregates Prometheus data to InfluxDB)
4. InfluxDB
5. **This tool aggregates further, from InfluxDB to BigQuery**

![image](https://user-images.githubusercontent.com/13122042/226832418-4c922503-4fca-4fbd-9b40-f38f1a37f990.png)


### Code Structure

How the tool and packages are structured:

1. Entrypoint: the `cmd/planet-federator-influxdb-to-bq` package. It contains the standalone analysis logic and BQ storage backend implementation.
2. Dependency package: `federator/influxdb/query`, to access the InfluxDB data via compatible API version.
3. Cron mechanism: Similar to planet-federator, it periodically triggers the analysis.

### InfluxDB to BQ Workflows

1. It queries InfluxDB using v1 API (using InfluxQL and not Flux), aligning with current Federator InfluxDB version.
2. It processes the data aggregations.
3. It stores the results in BigQuery Tables (i.e. traffic and dependency tables).

### Analysis 01: Traffic Data (Hourly)

Service-to-service traffic bandwidth in bits (1h min, max, & avg).

In this example below, we see:

1. It's an ingress traffic row
2. The `myservice` machine received ingress traffic from `other-service` machine
3. Total network bandwidth used for this ingress traffic on avg. was 6.823 Kbit/s 

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

![image](https://user-images.githubusercontent.com/13122042/226832942-9c5813ab-fe37-4ad8-82b1-1d6910d1c78f.png)

### Analysis 02: Dependency Data (Daily)

Service-to-service dependency, along with what the process_name & port that's getting accessed by downstreams.

In this example below, we see:

1. It's an upstream dependency row
2. The `myservice-process-name` process from `myservice` machine had accessed an upstream machine called `other-service` on port tcp:80.
3. Since it's an **upstream** data, it's normal for `local_hostgroup_address_port` to be null. The ephemeral port used by `myservice-process-name` to call the upstream may have been released at the time of the planet-exporter scrape.
4. If this was a **downstream** data, the `local_hostgroup_address` port would show the port that had been accessed by the downstream machine.

| Field                          | Example Value                  |
|--------------------------------|--------------------------------|
| inventory_date                 | 2023-03-20 12:51:00.000275 UTC |
|  dependency_direction          | upstream                       |
|  protocol                      | tcp                            |
|  local_hostgroup_process_name  | myservice-process-name         |
|  local_hostgroup               | myservice                      |
|  local_hostgroup_address       | myservice.service.consul       |
|  local_hostgroup_address_port  |                                |
|  remote_hostgroup              | other-service                  |
|  remote_hostgroup_address      | other-service.service.consul   |
|  remote_hostgroup_address_port | 80                             |

![image](https://user-images.githubusercontent.com/13122042/226833082-492b10db-ba0d-491d-b102-487ebc8e8689.png)
