package config

import (
	"os"
	"path/filepath"
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
