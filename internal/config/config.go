// Package config loads runtime configuration for the cluster-smi binaries.
//
// Configuration is read from the YAML file pointed to by the
// CLUSTER_SMI_CONFIG_PATH environment variable. If the variable is unset
// or empty, the system-wide files in DefaultConfigPaths are tried instead.
// If neither file can be read or parsed, the defaults defined below are used.
//
// The config file may name a directory with the includepath key. The
// .yaml and .yml files in that directory are then read in alphabetical
// order, each overriding the values set before it.
package config

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

// EnvConfigPath is the environment variable pointing to a YAML config file.
const EnvConfigPath = "CLUSTER_SMI_CONFIG_PATH"

// DefaultConfigPaths are the system-wide config files tried in order when
// CLUSTER_SMI_CONFIG_PATH is unset or empty. The first existing one is used.
var DefaultConfigPaths = []string{
	"/etc/cluster-smi/cluster-smi.yaml",
	"/etc/cluster-smi/cluster-smi.yml",
}

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
	Tick        int    `yaml:"tick"`        // tick (in seconds) between receiving data
	Timeout     int    `yaml:"timeout"`     // threshold (in seconds) after a node is considered/displayed as offline
	RouterIp    string `yaml:"router_ip"`   // ip of cluster-smi-router
	IncludePath string `yaml:"includepath"` // directory of further config files overriding this one
	Ports       struct {
		Nodes   string `yaml:"nodes"`   // port of cluster-smi-router, which nodes send to
		Clients string `yaml:"clients"` // port of cluster-smi-router, where clients subscribe to
		Metrics string `yaml:"metrics"` // port of cluster-smi-router serving Prometheus metrics (empty disables)
	} `yaml:"ports"`

	source   string   // file the configuration was read from, empty when defaults are used
	includes []string // files read from IncludePath, in the order they were applied
}

// Load returns a Config populated with defaults, optionally overridden by
// the YAML file at CLUSTER_SMI_CONFIG_PATH or, when that variable is unset,
// by the first existing file in DefaultConfigPaths, and then by the files
// in its IncludePath. If the file cannot be read or parsed, the defaults
// are kept.
func Load() Config {
	c := defaults()

	paths := DefaultConfigPaths
	explicit := os.Getenv(EnvConfigPath) != ""
	if explicit {
		paths = []string{os.Getenv(EnvConfigPath)}
	}

	var path string
	var yamlFile []byte
	found := false
	for _, path = range paths {
		var err error
		if yamlFile, err = os.ReadFile(path); err == nil {
			found = true
			break
		}
		// A missing system-wide file is the normal case, so only
		// complain when the path was requested explicitly.
		if explicit || !os.IsNotExist(err) {
			log.Printf("Could not read %s: %v - using default configuration", path, err)
			return c
		}
	}
	if !found {
		return c
	}

	parsed := defaults()
	if err := yaml.Unmarshal(yamlFile, &parsed); err != nil {
		log.Printf("Could not parse %s: %v - using default configuration", path, err)
		return c
	}

	parsed.source = path
	parsed.applyIncludes()
	return parsed
}

// applyIncludes overrides c with the .yaml and .yml files in c.IncludePath,
// in alphabetical order. A relative IncludePath is resolved against the
// directory of the main config file. Included files cannot include further
// files, and a file that cannot be read or parsed is skipped.
func (c *Config) applyIncludes() {
	dir := c.IncludePath
	if dir == "" {
		return
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(filepath.Dir(c.source), dir)
	}

	// os.ReadDir returns the entries sorted by filename
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("Could not read includepath %s: %v", dir, err)
		return
	}

	for _, entry := range entries {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if entry.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())

		yamlFile, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Could not read %s: %v - skipping", path, err)
			continue
		}

		// decode into a copy, so a broken file leaves no partial overrides
		parsed := *c
		if err := yaml.Unmarshal(yamlFile, &parsed); err != nil {
			log.Printf("Could not parse %s: %v - skipping", path, err)
			continue
		}
		parsed.IncludePath = c.IncludePath
		parsed.includes = append(c.includes, path)
		*c = parsed
	}
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
	for _, include := range c.includes {
		log.Println("  included", include)
	}

	log.Println("  Tick:", c.Tick)
	log.Println("  Timeout:", c.Timeout)
	log.Println("  RouterIp:", c.RouterIp)
	if c.IncludePath != "" {
		log.Println("  IncludePath:", c.IncludePath)
	}
	log.Println("  Ports:")
	log.Println("    Nodes:", c.Ports.Nodes)
	log.Println("    Clients:", c.Ports.Clients)
	log.Println("    Metrics:", c.Ports.Metrics)
}
