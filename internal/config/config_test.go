package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, path, routerIP string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("router_ip: "+routerIP+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDefaultPaths(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "cluster-smi.yaml")
	ymlPath := filepath.Join(dir, "cluster-smi.yml")

	saved := DefaultConfigPaths
	DefaultConfigPaths = []string{yamlPath, ymlPath}
	t.Cleanup(func() { DefaultConfigPaths = saved })
	t.Setenv(EnvConfigPath, "")

	if got := Load(); got.RouterIp != DefaultRouterIP || got.source != "" {
		t.Errorf("no file: got router %q from %q, want defaults", got.RouterIp, got.source)
	}

	writeConfig(t, ymlPath, "10.0.0.2")
	if got := Load(); got.RouterIp != "10.0.0.2" || got.source != ymlPath {
		t.Errorf("only .yml: got router %q from %q", got.RouterIp, got.source)
	}

	writeConfig(t, yamlPath, "10.0.0.1")
	if got := Load(); got.RouterIp != "10.0.0.1" || got.source != yamlPath {
		t.Errorf(".yaml and .yml: got router %q from %q, want .yaml to win", got.RouterIp, got.source)
	}
}

func TestLoadExplicitPath(t *testing.T) {
	dir := t.TempDir()

	saved := DefaultConfigPaths
	DefaultConfigPaths = []string{filepath.Join(dir, "cluster-smi.yaml")}
	t.Cleanup(func() { DefaultConfigPaths = saved })
	writeConfig(t, DefaultConfigPaths[0], "10.0.0.1")

	// a missing explicit file must not fall back to the default paths
	t.Setenv(EnvConfigPath, filepath.Join(dir, "missing.yml"))
	if got := Load(); got.RouterIp != DefaultRouterIP {
		t.Errorf("missing explicit file: got router %q, want default", got.RouterIp)
	}

	explicit := filepath.Join(dir, "custom.yml")
	writeConfig(t, explicit, "10.0.0.3")
	t.Setenv(EnvConfigPath, explicit)
	if got := Load(); got.RouterIp != "10.0.0.3" || got.source != explicit {
		t.Errorf("explicit file: got router %q from %q", got.RouterIp, got.source)
	}
}

func TestLoadIncludePath(t *testing.T) {
	dir := t.TempDir()
	includeDir := filepath.Join(dir, "cluster-smi.d")
	if err := os.Mkdir(includeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		// main config, the relative includepath resolves against its directory
		filepath.Join(dir, "cluster-smi.yaml"): "router_ip: 10.0.0.1\ntick: 5\nincludepath: cluster-smi.d\nports:\n  nodes: \"7000\"\n",
		// applied in alphabetical order: 20-* overrides 10-*
		filepath.Join(includeDir, "10-router.yml"):  "router_ip: 10.0.0.2\nports:\n  metrics: \"9082\"\n",
		filepath.Join(includeDir, "20-router.yaml"): "router_ip: 10.0.0.3\n",
		// cannot include further files
		filepath.Join(includeDir, "30-include.yaml"): "includepath: /nonexistent\n",
		// skipped: broken, or not a yaml file
		filepath.Join(includeDir, "40-broken.yaml"): "timeout: [not an int\n",
		filepath.Join(includeDir, "50-notes.txt"):   "router_ip: 10.0.0.9\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(EnvConfigPath, filepath.Join(dir, "cluster-smi.yaml"))

	got := Load()

	if got.RouterIp != "10.0.0.3" {
		t.Errorf("RouterIp = %q, want the last include to win", got.RouterIp)
	}
	if got.Tick != 5 || got.Ports.Nodes != "7000" {
		t.Errorf("Tick, Ports.Nodes = %d, %q, want the main config values kept", got.Tick, got.Ports.Nodes)
	}
	if got.Ports.Metrics != "9082" || got.Ports.Clients != DefaultPortClients {
		t.Errorf("Ports.Metrics, Ports.Clients = %q, %q, want the include and default values", got.Ports.Metrics, got.Ports.Clients)
	}
	if got.Timeout != DefaultTimeoutSeconds {
		t.Errorf("Timeout = %d, want the broken include skipped", got.Timeout)
	}
	if got.IncludePath != "cluster-smi.d" {
		t.Errorf("IncludePath = %q, want it not overridden by an include", got.IncludePath)
	}

	want := []string{
		filepath.Join(includeDir, "10-router.yml"),
		filepath.Join(includeDir, "20-router.yaml"),
		filepath.Join(includeDir, "30-include.yaml"),
	}
	if strings.Join(got.includes, ",") != strings.Join(want, ",") {
		t.Errorf("includes = %v, want %v", got.includes, want)
	}
}
