// Package clusterapp contains helpers shared by the GPU-collecting binaries
// (cluster-smi-node and cluster-smi-local) for initializing and refreshing
// per-node GPU state via NVML.
package clusterapp

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"strconv"
	"time"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/patwie/cluster-smi/cluster"
	"github.com/patwie/cluster-smi/proc"
)

// InitNVML loads libnvidia-ml and initializes NVML. A successful call must be
// paired with ShutdownNVML.
func InitNVML() error {
	if ret := nvml.Init(); ret != nvml.SUCCESS {
		return fmt.Errorf("nvml: init: %s", nvml.ErrorString(ret))
	}
	return nil
}

// ShutdownNVML releases the resources acquired by InitNVML. Failures are logged
// rather than returned, so it is usable with defer.
func ShutdownNVML() {
	if ret := nvml.Shutdown(); ret != nvml.SUCCESS {
		log.Printf("failed shutting down NVML: %s", nvml.ErrorString(ret))
	}
}

// nvmlDevices returns a handle for every NVML-visible GPU, in index order.
func nvmlDevices() ([]nvml.Device, error) {
	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("nvml: device count: %s", nvml.ErrorString(ret))
	}

	devices := make([]nvml.Device, 0, count)
	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf("nvml: handle for device %d: %s", i, nvml.ErrorString(ret))
		}
		devices = append(devices, device)
	}

	return devices, nil
}

// FetchCluster refreshes every node in the given cluster.
func FetchCluster(c *cluster.Cluster) {
	for i := range c.Nodes {
		FetchNode(&c.Nodes[i])
	}
}

// InitNode populates a Node with hostname, boot time, clock-tick info and
// allocates a Device entry for each NVML-visible GPU.
func InitNode(n *cluster.Node) {
	name, err := os.Hostname()
	if err != nil {
		log.Fatalf("failed to read hostname: %s", err.Error())
	}
	n.Name = name
	n.Time = time.Now()

	bootTime, _ := proc.BootTime()
	n.BootTime = bootTime
	n.ClockTicks = proc.ClockTicks()

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		log.Printf("failed to count NVML devices: %s", nvml.ErrorString(ret))
		return
	}

	for i := 0; i < count; i++ {
		n.Devices = append(n.Devices, cluster.Device{})
	}
}

// FetchNode refreshes a Node's GPU state from NVML and /proc.
func FetchNode(n *cluster.Node) {
	n.Time = time.Now()

	bootTime, _ := proc.BootTime()
	n.BootTime = bootTime

	currentTime := proc.TimeOfDay()

	devices, err := nvmlDevices()
	if err != nil {
		log.Printf("failed to enumerate NVML devices: %s", err.Error())
		return
	}

	for idx, device := range devices {
		name, _ := device.GetName()
		utilization, _ := device.GetUtilizationRates()
		powerUsage, _ := device.GetPowerUsage()
		fanSpeed, _ := device.GetFanSpeed()
		tempc, _ := device.GetTemperature(nvml.TEMPERATURE_GPU)

		// read memory usage
		var used, free, total int64
		memPercent := 0
		if meminfo, ret := device.GetMemoryInfo(); ret != nvml.SUCCESS {
			log.Printf("failed to read memory info for device %d: %s", idx, nvml.ErrorString(ret))
		} else {
			used, free, total = int64(meminfo.Used), int64(meminfo.Free), int64(meminfo.Total)
			if total > 0 {
				memPercent = int(used * 100 / total)
			}
		}

		// read processes
		deviceProcs, ret := device.GetComputeRunningProcesses()
		if ret != nvml.SUCCESS {
			log.Printf("failed to read process info for device %d: %s", idx, nvml.ErrorString(ret))
		}

		// collect all process information
		var processes []cluster.Process

		for _, deviceProc := range deviceProcs {
			pid := int(deviceProc.Pid)
			if pid == 0 {
				continue
			}

			// the process may have exited, or live in another PID namespace
			name := "unknown"
			var runTime int64
			if pidInfo, err := proc.InfoFromPid(pid); err == nil {
				name = pidInfo.Command
				runTime = (currentTime - n.BootTime) - (pidInfo.StartTime / n.ClockTicks)
			}

			username := "unknown"
			if uid, err := proc.UIDFromPID(pid); err == nil {
				if usr, err := user.LookupId(strconv.Itoa(uid)); err == nil {
					username = usr.Username
				}
			}

			extendedCommand := proc.CmdFromPID(pid)

			processes = append(processes, cluster.Process{
				Pid:             pid,
				UsedGpuMemory:   int64(deviceProc.UsedGpuMemory),
				Name:            name,
				Username:        username,
				RunTime:         runTime,
				ExtendedCommand: extendedCommand,
			})
		}

		n.Devices[idx].Id = idx
		n.Devices[idx].Name = name
		n.Devices[idx].Utilization = int(utilization.Gpu)
		n.Devices[idx].MemoryUtilization = cluster.Memory{Used: used, Free: free, Total: total, Percentage: memPercent}
		n.Devices[idx].FanSpeed = int(fanSpeed)
		// nvmlDeviceGetPowerUsage reports milliwatts; convert to watts
		n.Devices[idx].PowerUsage = int(powerUsage / 1000)
		n.Devices[idx].Temperature = int(tempc)
		n.Devices[idx].Processes = processes
	}
}
