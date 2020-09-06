## Planet Exporter

> Serves as a starting point to write your very specific use-cases that are not available in any exporters.

Simple discovery space-ship for your ~~infrastructure~~ planetary ecosystem across the universe.

Measure an environment's potential to maintain ~~services~~ life.

### Examples

#### Processes that are listening on any server port

Shows all processes that are opening port and listening

```
# Processes that are holding the file descriptors
planet_serving{protocol="tcp", name="nginx", addr="10.12.3.4", port="80"} 1
planet_serving{protocol="tcp", name="asdf", addr="10.12.3.4", port="8080"} 1
planet_serving{protocol="tcp", name="node_exporter", addr="0.0.0.0", port="9100"} 1
planet_serving{protocol="udp", name="datadog-agent", addr="0.0.0.0", port="8125"} 1
planet_serving{protocol="udp", name="dnsmasq", addr="127.0.0.1", port="53"} 1
planet_serving{protocol="tcp", name="dnsmasq", addr="127.0.0.1", port="53"} 1
```

#### Upstream and downstream dependency connections

Dependency information of a node. Useful to generate a simple dependency graph.

```
# This node depends on xyz-redis and shows bytes transferred counter
planet_downstream{protocol="tcp", name="some-api", domain="some-api.service.dc.consul", port="80", direction="in"} 1234
planet_downstream{protocol="tcp", name="some-api", domain="some-api.service.dc.consul", port="80", direction="out"} 1234

# Node asdf depends on this node snd shows bytes transferred counter
planet_upstream{protocol="tcp", name="some-redis", domain="some-redis.service.dc.consul", port="6379", direction="in"} 321415
planet_upstream{protocol="tcp", name="some-redis", domain="some-redis.service.dc.consul", port="6379", direction="out"} 2415
```

#### Expose your data in Golang

Want to write your own exporter in Go instead of using node_exporter's `--collector.textfile.directory`? This is your alternative in Go.

Simply fork and write your own `./collector/xyz.go` here.

#### Customise and wrap other available exporters data

You can even query other local exporters data in the node and reshape them into something you need.

```
# Show postgres version from postgres_exporter (localhost:9187) and query upstream for potential update
planet_postgres_available 1
planet_postgres_version_update{current="v11-xyz", latest="v12-xyz"}
```
