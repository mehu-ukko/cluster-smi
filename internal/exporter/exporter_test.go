package exporter

import (
	"strings"
	"testing"
	"time"

	"github.com/mehu-ukko/cluster-smi/cluster"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestCollect(t *testing.T) {
	now := time.Unix(1700000000, 0)

	nodes := []cluster.Node{
		{
			Name: "node01",
			Time: now.Add(-5 * time.Second),
			Devices: []cluster.Device{{
				Id:                0,
				Name:              "TITAN Xp",
				Utilization:       39,
				MemoryUtilization: cluster.Memory{Used: 3000, Total: 12000},
				FanSpeed:          40,
				Temperature:       65,
				PowerUsage:        120,
				Processes: []cluster.Process{
					{Pid: 1, Username: "john", UsedGpuMemory: 1000},
					{Pid: 2, Username: "john", UsedGpuMemory: 1500},
					{Pid: 3, Username: "jane", UsedGpuMemory: 500},
				},
			}, {
				Id:                1,
				Name:              "TITAN Xp",
				MemoryUtilization: cluster.Memory{Total: 12000},
			}},
		},
		{
			Name: "node02",
			Time: now.Add(-time.Hour),
			Devices: []cluster.Device{{
				Id:          0,
				Name:        "TITAN Xp",
				Utilization: 99,
			}},
		},
	}

	c := New(func() []cluster.Node { return nodes }, 180*time.Second)
	c.now = func() time.Time { return now }

	expected := `
# HELP cluster_smi_gpu_available Whether no processes are running on the GPU (1) or not (0).
# TYPE cluster_smi_gpu_available gauge
cluster_smi_gpu_available{gpu="0",name="TITAN Xp",node="node01"} 0
cluster_smi_gpu_available{gpu="1",name="TITAN Xp",node="node01"} 1
# HELP cluster_smi_gpu_busy Whether processes are running on the GPU (1) or not (0).
# TYPE cluster_smi_gpu_busy gauge
cluster_smi_gpu_busy{gpu="0",name="TITAN Xp",node="node01"} 1
cluster_smi_gpu_busy{gpu="1",name="TITAN Xp",node="node01"} 0
# HELP cluster_smi_gpu_fan_speed_percent GPU fan speed in percent.
# TYPE cluster_smi_gpu_fan_speed_percent gauge
cluster_smi_gpu_fan_speed_percent{gpu="0",name="TITAN Xp",node="node01"} 40
cluster_smi_gpu_fan_speed_percent{gpu="1",name="TITAN Xp",node="node01"} 0
# HELP cluster_smi_gpu_memory_total_bytes Total GPU memory in bytes.
# TYPE cluster_smi_gpu_memory_total_bytes gauge
cluster_smi_gpu_memory_total_bytes{gpu="0",name="TITAN Xp",node="node01"} 12000
cluster_smi_gpu_memory_total_bytes{gpu="1",name="TITAN Xp",node="node01"} 12000
# HELP cluster_smi_gpu_memory_used_bytes Used GPU memory in bytes.
# TYPE cluster_smi_gpu_memory_used_bytes gauge
cluster_smi_gpu_memory_used_bytes{gpu="0",name="TITAN Xp",node="node01"} 3000
cluster_smi_gpu_memory_used_bytes{gpu="1",name="TITAN Xp",node="node01"} 0
# HELP cluster_smi_gpu_power_watts GPU power usage in watts.
# TYPE cluster_smi_gpu_power_watts gauge
cluster_smi_gpu_power_watts{gpu="0",name="TITAN Xp",node="node01"} 120
cluster_smi_gpu_power_watts{gpu="1",name="TITAN Xp",node="node01"} 0
# HELP cluster_smi_gpu_processes Number of processes running on the GPU.
# TYPE cluster_smi_gpu_processes gauge
cluster_smi_gpu_processes{gpu="0",name="TITAN Xp",node="node01"} 3
cluster_smi_gpu_processes{gpu="1",name="TITAN Xp",node="node01"} 0
# HELP cluster_smi_gpu_temperature_celsius GPU temperature in degrees Celsius.
# TYPE cluster_smi_gpu_temperature_celsius gauge
cluster_smi_gpu_temperature_celsius{gpu="0",name="TITAN Xp",node="node01"} 65
cluster_smi_gpu_temperature_celsius{gpu="1",name="TITAN Xp",node="node01"} 0
# HELP cluster_smi_gpu_user_memory_used_bytes GPU memory in bytes used by the processes of a user.
# TYPE cluster_smi_gpu_user_memory_used_bytes gauge
cluster_smi_gpu_user_memory_used_bytes{gpu="0",name="TITAN Xp",node="node01",user="jane"} 500
cluster_smi_gpu_user_memory_used_bytes{gpu="0",name="TITAN Xp",node="node01",user="john"} 2500
# HELP cluster_smi_gpu_utilization_percent GPU utilization in percent.
# TYPE cluster_smi_gpu_utilization_percent gauge
cluster_smi_gpu_utilization_percent{gpu="0",name="TITAN Xp",node="node01"} 39
cluster_smi_gpu_utilization_percent{gpu="1",name="TITAN Xp",node="node01"} 0
# HELP cluster_smi_node_last_seen_timestamp_seconds Unix time of the last report received from the node.
# TYPE cluster_smi_node_last_seen_timestamp_seconds gauge
cluster_smi_node_last_seen_timestamp_seconds{node="node01"} 1.699999995e+09
cluster_smi_node_last_seen_timestamp_seconds{node="node02"} 1.6999964e+09
# HELP cluster_smi_node_up Whether the node reported within the configured timeout (1) or not (0).
# TYPE cluster_smi_node_up gauge
cluster_smi_node_up{node="node01"} 1
cluster_smi_node_up{node="node02"} 0
`

	if err := testutil.CollectAndCompare(c, strings.NewReader(expected)); err != nil {
		t.Fatal(err)
	}
}
