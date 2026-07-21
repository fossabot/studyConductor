package pkg

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConfigMapAccessors(t *testing.T) {
	cfgMap := ConfigMap{
		"string": "value",
		"slice":  []interface{}{"one", "two"},
		"map":    ConfigMap{"a": "1", "b": 2},
	}

	if got := cfgMap.GetString("string"); got != "value" {
		t.Fatalf("GetString() = %q, want %q", got, "value")
	}

	if got := cfgMap.GetStringSlice("slice"); !reflect.DeepEqual(got, []string{"one", "two"}) {
		t.Fatalf("GetStringSlice() = %#v, want %#v", got, []string{"one", "two"})
	}

	if got := cfgMap.GetMap("map"); !reflect.DeepEqual(got, ConfigMap{"a": "1", "b": 2}) {
		t.Fatalf("GetMap() = %#v, want %#v", got, ConfigMap{"a": "1", "b": 2})
	}

	if got := cfgMap.GetStringMap("map"); !reflect.DeepEqual(got, map[string]string{"a": "1", "b": "2"}) {
		t.Fatalf("GetStringMap() = %#v, want %#v", got, map[string]string{"a": "1", "b": "2"})
	}
}

func TestConfigMapGetStringSliceNil(t *testing.T) {
	cfgMap := ConfigMap{}

	if got := cfgMap.GetStringSlice("missing"); got != nil {
		t.Fatalf("GetStringSlice() = %#v, want nil", got)
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
study:
  name: Study
  storage:
    local: "true"
    path: data
modules:
  - name: binary-step
    type: binary
    configuration:
      BINARY_PATH: /bin/echo
      BINARY_ARGS:
        - hello
`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	t.Setenv("CONFIG_FILE", configPath)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Study.Name != "Study" {
		t.Fatalf("LoadConfig() study name = %q, want %q", cfg.Study.Name, "Study")
	}
	if len(cfg.Modules) != 1 {
		t.Fatalf("LoadConfig() modules = %d, want 1", len(cfg.Modules))
	}
}
