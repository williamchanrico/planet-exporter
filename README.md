## Planet Exporter

> Serves as a starting point to write very specific use-cases that are not available in any exporters.

Simple discovery space-ship for your ~~infrastructure~~ planetary ecosystem across the universe.

Measure an environment's potential to maintain ~~services~~ life.

### Expose your data in Golang

Want to write your own exporter in Go instead of using node_exporter's `--collector.textfile.directory`? This is your alternative in Go.

Write your own `./collector/xyz.go` here.

Expensive metrics that runs in background periodically are put in `./collector/task/xyz/xyz.go`

* The `task/*` packages are the crew that does expensive task behind the scene and prepare the data for `collector` package.
* The `collector` package calls one/more `task/*` packages if it needs expensive metrics instead of preparing them on-the-fly.

#### Example Tasks

##### Darkstat

[Darkstat](https://unix4lyfe.org/darkstat/) captures network traffic, calculates statistics about usage, and serves reports over HTTP.

Data from darkstat can be leveraged for network dependencies capture.
That means we'll have to install darkstat along with planet-exporter.
