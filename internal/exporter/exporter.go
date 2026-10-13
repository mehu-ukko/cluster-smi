// Package exporter exposes the cluster state collected by cluster-smi-router
// as Prometheus metrics.
package exporter

import (
	"strconv"
	"time"

	"github.com/mehu-ukko/cluster-smi/cluster"
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "cluster_smi"

var (
	nodeLabels   = []string{"node"}
	deviceLabels = []string{"node", "gpu", "name"}

	nodeUp = prometheus.NewDesc(namespace+"_node_up",
		"Whether the node reported within the configured timeout (1) or not (0).", nodeLabels, nil)
	nodeLastSeen = prometheus.NewDesc(namespace+"_node_last_seen_timestamp_seconds",
		"Unix time of the last report received from the node.", nodeLabels, nil)

	gpuUtilization = prometheus.NewDesc(namespace+"_gpu_utilization_percent",
		"GPU utilization in percent.", deviceLabels, nil)
	gpuMemoryUsed = prometheus.NewDesc(namespace+"_gpu_memory_used_bytes",
		"Used GPU memory in bytes.", deviceLabels, nil)
	gpuMemoryTotal = prometheus.NewDesc(namespace+"_gpu_memory_total_bytes",
		"Total GPU memory in bytes.", deviceLabels, nil)
	gpuTemperature = prometheus.NewDesc(namespace+"_gpu_temperature_celsius",
		"GPU temperature in degrees Celsius.", deviceLabels, nil)
	gpuFanSpeed = prometheus.NewDesc(namespace+"_gpu_fan_speed_percent",
		"GPU fan speed in percent.", deviceLabels, nil)
	gpuPower = prometheus.NewDesc(namespace+"_gpu_power_watts",
		"GPU power usage in watts.", deviceLabels, nil)
	gpuProcesses = prometheus.NewDesc(namespace+"_gpu_processes",
		"Number of processes running on the GPU.", deviceLabels, nil)
	gpuBusy = prometheus.NewDesc(namespace+"_gpu_busy",
		"Whether processes are running on the GPU (1) or not (0).", deviceLabels, nil)
	gpuAvailable = prometheus.NewDesc(namespace+"_gpu_available",
		"Whether no processes are running on the GPU (1) or not (0).", deviceLabels, nil)
	gpuUserMemoryUsed = prometheus.NewDesc(namespace+"_gpu_user_memory_used_bytes",
		"GPU memory in bytes used by the processes of a user.", append(deviceLabels, "user"), nil)
)

// Collector is a prometheus.Collector reading the cluster state on every scrape.
type Collector struct {
	snapshot func() []cluster.Node
	timeout  time.Duration
	now      func() time.Time
}

// New returns a Collector taking the current nodes from snapshot. Nodes
// whose last report is older than timeout are reported as down and their
// devices are omitted.
func New(snapshot func() []cluster.Node, timeout time.Duration) *Collector {
	return &Collector{snapshot: snapshot, timeout: timeout, now: time.Now}
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{
		nodeUp, nodeLastSeen,
		gpuUtilization, gpuMemoryUsed, gpuMemoryTotal, gpuTemperature,
		gpuFanSpeed, gpuPower, gpuProcesses, gpuBusy, gpuAvailable, gpuUserMemoryUsed,
	} {
		ch <- d
	}
}

// Collect implements prometheus.Collector.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	now := c.now()

	for _, n := range c.snapshot() {
		online := now.Sub(n.Time) <= c.timeout

		up := 0.0
		if online {
			up = 1
		}
		ch <- prometheus.MustNewConstMetric(nodeUp, prometheus.GaugeValue, up, n.Name)
		ch <- prometheus.MustNewConstMetric(nodeLastSeen, prometheus.GaugeValue,
			float64(n.Time.UnixNano())/1e9, n.Name)

		// let the series of offline nodes go stale instead of repeating old values
		if !online {
			continue
		}

		for _, d := range n.Devices {
			labels := []string{n.Name, strconv.Itoa(d.Id), d.Name}
			gauge := func(desc *prometheus.Desc, v float64) {
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, labels...)
			}

			gauge(gpuUtilization, float64(d.Utilization))
			gauge(gpuMemoryUsed, float64(d.MemoryUtilization.Used))
			gauge(gpuMemoryTotal, float64(d.MemoryUtilization.Total))
			gauge(gpuTemperature, float64(d.Temperature))
			gauge(gpuFanSpeed, float64(d.FanSpeed))
			gauge(gpuPower, float64(d.PowerUsage))
			gauge(gpuProcesses, float64(len(d.Processes)))

			busy := 0.0
			if len(d.Processes) > 0 {
				busy = 1
			}
			gauge(gpuBusy, busy)
			gauge(gpuAvailable, 1-busy)

			// aggregate per user: a pid label would explode the series cardinality
			userMemory := make(map[string]int64)
			for _, p := range d.Processes {
				userMemory[p.Username] += p.UsedGpuMemory
			}
			for user, used := range userMemory {
				ch <- prometheus.MustNewConstMetric(gpuUserMemoryUsed, prometheus.GaugeValue,
					float64(used), append(labels, user)...)
			}
		}
	}
}
