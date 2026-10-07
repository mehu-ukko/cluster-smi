// Package config loads runtime configuration for the cluster-smi binaries.
//
// Configuration is read from the YAML file pointed to by the
// CLUSTER_SMI_CONFIG_PATH environment variable. If the variable is unset
// or empty, the system-wide file at DefaultConfigPath is tried instead.
// If neither file can be read or parsed, the defaults defined below are used.
package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

// EnvConfigPath is the environment variable pointing to a YAML config file.
const EnvConfigPath = "CLUSTER_SMI_CONFIG_PATH"

// DefaultConfigPath is the system-wide config file used when
// CLUSTER_SMI_CONFIG_PATH is unset or empty.
const DefaultConfigPath = "/etc/cluster-smi/cluster-smi.yaml"

// Default values applied when no configuration file is provided.
const (
	// DefaultRouterIP is the address of the cluster-smi-router that
	// nodes push to and clients connect to.
	DefaultRouterIP = "127.0.0.1"

	// DefaultTickSeconds is the polling interval (in seconds) used by
	// nodes to refresh GPU state and by clients to refresh their view.
	DefaultTickSeconds = 3

	// DefaultTimeoutSeconds is the threshold after which a node that has
	// not reported recently is considered offline.
	DefaultTimeoutSeconds = 180

	// DefaultPortNodes is the router port that nodes push their state to.
	DefaultPortNodes = "9080"

	// DefaultPortClients is the router port that clients subscribe to.
	DefaultPortClients = "9081"

	// DefaultPortMetrics is the router port serving Prometheus metrics.
	// Empty disables the exporter.
	DefaultPortMetrics = ""
)

// Config holds the runtime configuration for the cluster-smi binaries.
type Config struct {
	Tick     int    `yaml:"tick"`      // tick (in seconds) between receiving data
	Timeout  int    `yaml:"timeout"`   // threshold (in seconds) after a node is considered/displayed as offline
	RouterIp string `yaml:"router_ip"` // ip of cluster-smi-router
	Ports    struct {
		Nodes   string `yaml:"nodes"`   // port of cluster-smi-router, which nodes send to
		Clients string `yaml:"clients"` // port of cluster-smi-router, where clients subscribe to
		Metrics string `yaml:"metrics"` // port of cluster-smi-router serving Prometheus metrics (empty disables)
	} `yaml:"ports"`

	source string // file the configuration was read from, empty when defaults are used
}

// Load returns a Config populated with defaults, optionally overridden by
// the YAML file at CLUSTER_SMI_CONFIG_PATH or, when that variable is unset,
// by DefaultConfigPath. If the file cannot be read or parsed, the defaults
// are kept.
func Load() Config {
	c := defaults()

	path := os.Getenv(EnvConfigPath)
	explicit := path != ""
	if !explicit {
		path = DefaultConfigPath
	}

	yamlFile, err := os.ReadFile(path)
	if err != nil {
		// A missing system-wide file is the normal case, so only
		// complain when the path was requested explicitly.
		if explicit || !os.IsNotExist(err) {
			log.Printf("Could not read %s: %v - using default configuration", path, err)
		}
		return c
	}

	parsed := defaults()
	if err := yaml.Unmarshal(yamlFile, &parsed); err != nil {
		log.Printf("Could not parse %s: %v - using default configuration", path, err)
		return c
	}

	parsed.source = path
	return parsed
}

// defaults returns a Config populated with the built-in default values.
func defaults() Config {
	c := Config{}

	c.RouterIp = DefaultRouterIP
	c.Tick = DefaultTickSeconds
	c.Timeout = DefaultTimeoutSeconds
	c.Ports.Nodes = DefaultPortNodes
	c.Ports.Clients = DefaultPortClients
	c.Ports.Metrics = DefaultPortMetrics

	return c
}

// Print logs the active configuration values.
func (c Config) Print() {
	if c.source != "" {
		log.Println("Read configuration from", c.source)
	} else {
		log.Println("use default configuration")
	}

	log.Println("  Tick:", c.Tick)
	log.Println("  Timeout:", c.Timeout)
	log.Println("  RouterIp:", c.RouterIp)
	log.Println("  Ports:")
	log.Println("    Nodes:", c.Ports.Nodes)
	log.Println("    Clients:", c.Ports.Clients)
	log.Println("    Metrics:", c.Ports.Metrics)
}
