package pkg

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDetect(t *testing.T) {
	originalCombinedOutput := lsblkCombinedOutput
	originalCfg := Cfg
	t.Cleanup(func() {
		lsblkCombinedOutput = originalCombinedOutput
		Cfg = originalCfg
	})

	Cfg = &Config{Study: Study{Storage: ConfigMap{"id": "match"}}}
	lsblkCombinedOutput = func() ([]byte, error) {
		return []byte(`{"blockdevices":[{"name":"sda","children":[{"name":"sda1","type":"part","mountpoint":"/mnt/usb","uuid":"match"}]}]}`), nil
	}

	got, err := Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if got == nil || got.Uuid != "match" || got.Name != "sda1" {
		t.Fatalf("Detect() = %#v, want matching drive", got)
	}
}

func TestDetectReturnsNilWhenNoMatch(t *testing.T) {
	originalCombinedOutput := lsblkCombinedOutput
	originalCfg := Cfg
	t.Cleanup(func() {
		lsblkCombinedOutput = originalCombinedOutput
		Cfg = originalCfg
	})

	Cfg = &Config{Study: Study{Storage: ConfigMap{"id": "missing"}}}
	lsblkCombinedOutput = func() ([]byte, error) {
		return []byte(`{"blockdevices":[{"name":"sda","children":[{"name":"sda1","type":"part","uuid":"other"}]}]}`), nil
	}

	got, err := Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if got != nil {
		t.Fatalf("Detect() = %#v, want nil", got)
	}
}

func TestGetStorageLocalAndUsbPaths(t *testing.T) {
	t.Run("local", func(t *testing.T) {
		originalDetect := detectDrive
		originalMkdirAll := mkdirAll
		originalCfg := Cfg
		t.Cleanup(func() {
			detectDrive = originalDetect
			mkdirAll = originalMkdirAll
			Cfg = originalCfg
		})

		detectDrive = func() (*BlockDevice, error) {
			t.Fatal("detectDrive should not be called for local storage")
			return nil, nil
		}
		calls := 0
		dir := t.TempDir()
		expectedPath := filepath.Join(dir, "data")
		mkdirAll = func(path string, perm os.FileMode) error {
			calls++
			if path != expectedPath {
				return errors.New("unexpected path")
			}
			return nil
		}
		cfg := &Config{Study: Study{Storage: ConfigMap{"local": "true", "path": expectedPath}}}

		got, err := GetStorage(cfg)
		if err != nil {
			t.Fatalf("GetStorage() error = %v", err)
		}
		if got != expectedPath {
			t.Fatalf("GetStorage() = %q, want %q", got, expectedPath)
		}
		if calls != 1 {
			t.Fatalf("mkdirAll called %d times, want 1", calls)
		}
	})

	t.Run("usb", func(t *testing.T) {
		originalDetect := detectDrive
		originalMkdirAll := mkdirAll
		originalCfg := Cfg
		t.Cleanup(func() {
			detectDrive = originalDetect
			mkdirAll = originalMkdirAll
			Cfg = originalCfg
		})

		detectDrive = func() (*BlockDevice, error) {
			return &BlockDevice{Name: "sdb1", Mountpoint: "/mnt/usb"}, nil
		}
		var gotPath string
		mkdirAll = func(path string, perm os.FileMode) error {
			gotPath = path
			return nil
		}
		cfg := &Config{Study: Study{Storage: ConfigMap{"local": "false", "path": "study-data"}}}

		got, err := GetStorage(cfg)
		if err != nil {
			t.Fatalf("GetStorage() error = %v", err)
		}
		want := filepath.Join("/mnt/usb", "study-data")
		if got != want {
			t.Fatalf("GetStorage() = %q, want %q", got, want)
		}
		if gotPath != want {
			t.Fatalf("mkdirAll path = %q, want %q", gotPath, want)
		}
	})
}

func TestGetStorageErrors(t *testing.T) {
	originalDetect := detectDrive
	originalMkdirAll := mkdirAll
	originalCfg := Cfg
	t.Cleanup(func() {
		detectDrive = originalDetect
		mkdirAll = originalMkdirAll
		Cfg = originalCfg
	})

	detectDrive = func() (*BlockDevice, error) {
		return nil, nil
	}
	mkdirAll = func(path string, perm os.FileMode) error {
		return nil
	}
	cfg := &Config{Study: Study{Storage: ConfigMap{"local": "false", "path": "data"}}}
	if _, err := GetStorage(cfg); err == nil {
		t.Fatal("GetStorage() error = nil, want error when no drive is found")
	}
}

func TestGetStorageUsesJoinedPathFromMountedDrive(t *testing.T) {
	originalDetect := detectDrive
	originalMkdirAll := mkdirAll
	originalCfg := Cfg
	t.Cleanup(func() {
		detectDrive = originalDetect
		mkdirAll = originalMkdirAll
		Cfg = originalCfg
	})

	detectDrive = func() (*BlockDevice, error) {
		return &BlockDevice{Name: "sdb1", Mountpoint: "/mnt/usb"}, nil
	}
	var paths []string
	mkdirAll = func(path string, perm os.FileMode) error {
		paths = append(paths, path)
		return nil
	}
	cfg := &Config{Study: Study{Storage: ConfigMap{"local": "false", "path": "nested/data"}}}

	got, err := GetStorage(cfg)
	if err != nil {
		t.Fatalf("GetStorage() error = %v", err)
	}
	want := filepath.Join("/mnt/usb", "nested/data")
	if got != want {
		t.Fatalf("GetStorage() = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(paths, []string{want}) {
		t.Fatalf("mkdirAll paths = %#v, want %#v", paths, []string{want})
	}
}
