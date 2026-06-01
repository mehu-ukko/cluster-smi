// Package clusterapp contains helpers shared by the GPU-collecting binaries
// (cluster-smi-node and cluster-smi-local) for initializing and refreshing
// per-node GPU state via NVML.
package clusterapp

import (
	"log"
	"os"
	"os/user"
	"strconv"
	"time"

	"github.com/patwie/cluster-smi/cluster"
	"github.com/patwie/cluster-smi/nvml"
	"github.com/patwie/cluster-smi/proc"
)

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

	devices, _ := nvml.GetDevices()

	for i := 0; i < len(devices); i++ {
		n.Devices = append(n.Devices, cluster.Device{0, "", 0, cluster.Memory{0, 0, 0, 0}, 0, 0, 0, nil})
	}
}

// FetchNode refreshes a Node's GPU state from NVML and /proc.
func FetchNode(n *cluster.Node) {
	devices, _ := nvml.GetDevices()
	n.Time = time.Now()

	bootTime, _ := proc.BootTime()
	n.BootTime = bootTime

	currentTime := proc.TimeOfDay()

	for idx, device := range devices {
		meminfo, _ := device.GetMemoryInfo()
		gpuPercent, _, _ := device.GetUtilization()
		memPercent := int(meminfo.Used / meminfo.Total)
		powerUsage, _ := device.GetPowerUsage()
		fanSpeed, _ := device.GetFanSpeed()
		tempc, _, _ := device.GetTemperature()

		// read processes
		deviceProcs, err := device.GetProcessInfo()
		if err != nil {
			log.Printf("failed to read process info for device %d: %s", idx, err.Error())
			continue
		}

		// collect all process information
		var processes []cluster.Process

		for i := 0; i < len(deviceProcs); i++ {
			if int(deviceProcs[i].Pid) == 0 {
				continue
			}

			pid := deviceProcs[i].Pid
			pidInfo := proc.InfoFromPid(pid)

			uid := proc.UIDFromPID(pid)
			usr, err := user.LookupId(strconv.Itoa(uid))

			username := "unknown"
			if err == nil {
				username = usr.Username
			}

			extendedCommand := proc.CmdFromPID(pid)

			processes = append(processes, cluster.Process{
				Pid:             pid,
				UsedGpuMemory:   deviceProcs[i].UsedGpuMemory,
				Name:            pidInfo.Command,
				Username:        username,
				RunTime:         (currentTime - n.BootTime) - (pidInfo.StartTime / n.ClockTicks),
				ExtendedCommand: extendedCommand,
			})
		}

		n.Devices[idx].Id = idx
		n.Devices[idx].Name = device.DeviceName
		n.Devices[idx].Utilization = gpuPercent
		n.Devices[idx].MemoryUtilization = cluster.Memory{meminfo.Used, meminfo.Free, meminfo.Total, memPercent}
		n.Devices[idx].FanSpeed = fanSpeed
		n.Devices[idx].PowerUsage = powerUsage
		n.Devices[idx].Temperature = tempc
		n.Devices[idx].Processes = processes
	}
}
