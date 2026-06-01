// Package config loads runtime configuration for the cluster-smi binaries.
//
// Configuration is read from the YAML file pointed to by the
// CLUSTER_SMI_CONFIG_PATH environment variable. If the variable is unset
// or empty, the defaults defined below are used.
package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

// EnvConfigPath is the environment variable pointing to a YAML config file.
const EnvConfigPath = "CLUSTER_SMI_CONFIG_PATH"

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
)

// Config holds the runtime configuration for the cluster-smi binaries.
type Config struct {
	Tick     int    `yaml:"tick"`      // tick (in seconds) between receiving data
	Timeout  int    `yaml:"timeout"`   // threshold (in seconds) after a node is considered/displayed as offline
	RouterIp string `yaml:"router_ip"` // ip of cluster-smi-router
	Ports    struct {
		Nodes   string `yaml:"nodes"`   // port of cluster-smi-router, which nodes send to
		Clients string `yaml:"clients"` // port of cluster-smi-router, where clients subscribe to
	} `yaml:"ports"`
}

// Load returns a Config populated with defaults, optionally overridden by
// the YAML file at CLUSTER_SMI_CONFIG_PATH.
func Load() Config {
	c := Config{}

	c.RouterIp = DefaultRouterIP
	c.Tick = DefaultTickSeconds
	c.Timeout = DefaultTimeoutSeconds
	c.Ports.Nodes = DefaultPortNodes
	c.Ports.Clients = DefaultPortClients

	path := os.Getenv(EnvConfigPath)
	if path == "" {
		return c
	}

	yamlFile, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Error: %v ", err)
	}

	if err := yaml.Unmarshal(yamlFile, &c); err != nil {
		log.Fatalf("Error in %s %v", path, err)
	}

	return c
}

// Print logs the active configuration values.
func (c Config) Print() {
	if path := os.Getenv(EnvConfigPath); path != "" {
		log.Println("Read configuration from", path)
	} else {
		log.Println("use default configuration")
	}

	log.Println("  Tick:", c.Tick)
	log.Println("  Timeout:", c.Timeout)
	log.Println("  RouterIp:", c.RouterIp)
	log.Println("  Ports:")
	log.Println("    Nodes:", c.Ports.Nodes)
	log.Println("    Clients:", c.Ports.Clients)
}
