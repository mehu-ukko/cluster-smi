// Package proc reads process and system information from /proc.
package proc

import (
	"strings"
	"time"

	"github.com/prometheus/procfs"
	"github.com/tklauser/go-sysconf"
)

// PIDInfo is a collection of properties for a process
type PIDInfo struct {
	PID       int
	Command   string
	UsedTime  int64
	StartTime int64 // clock ticks after system boot
}

// ClockTicks returns the number of clock ticks per second (USER_HZ)
func ClockTicks() int64 {
	hz, err := sysconf.Sysconf(sysconf.SC_CLK_TCK)
	if err != nil || hz <= 0 {
		return 100
	}
	return hz
}

// TimeOfDay returns current time-stamp
func TimeOfDay() int64 {
	return time.Now().Unix()
}

// BootTime returns the Unix timestamp at which the machine was started
func BootTime() (int64, error) {
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return 0, err
	}
	stat, err := fs.Stat()
	if err != nil {
		return 0, err
	}
	return int64(stat.BootTime), nil
}

// InfoFromPid returns name, used CPU time and start time of a process
func InfoFromPid(pid int) (PIDInfo, error) {
	p, err := procfs.NewProc(pid)
	if err != nil {
		return PIDInfo{PID: pid}, err
	}
	stat, err := p.Stat()
	if err != nil {
		return PIDInfo{PID: pid}, err
	}
	return PIDInfo{
		PID:       pid,
		Command:   stat.Comm,
		UsedTime:  int64(stat.UTime + stat.STime),
		StartTime: int64(stat.Starttime),
	}, nil
}

// UIDFromPID returns the real user id (UID) for a given process id (PID)
func UIDFromPID(pid int) (int, error) {
	p, err := procfs.NewProc(pid)
	if err != nil {
		return 0, err
	}
	status, err := p.NewStatus()
	if err != nil {
		return 0, err
	}
	return int(status.UIDs[0]), nil
}

// CmdFromPID returns the space-separated command line of a process
func CmdFromPID(pid int) string {
	p, err := procfs.NewProc(pid)
	if err != nil {
		return "unknown"
	}
	args, err := p.CmdLine()
	if err != nil || len(args) == 0 {
		return "unknown"
	}
	return strings.Join(args, " ")
}
