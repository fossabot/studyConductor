package step

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"studyConductor/pkg"
	"testing"
	"time"
)

func TestGetCmd(t *testing.T) {
	cases := []struct {
		name string
		cfg  pkg.ConfigMap
		want []string
		dir  string
	}{
		{
			name: "plain",
			cfg:  pkg.ConfigMap{"BINARY_PATH": "echo", "BINARY_ARGS": []any{"hello"}, "WORKING_DIR": "/tmp/work"},
			want: []string{"echo", "hello"},
			dir:  "/tmp/work",
		},
		{
			name: "terminal",
			cfg:  pkg.ConfigMap{"BINARY_PATH": "echo", "BINARY_ARGS": []any{"hello"}, "terminal": true},
			want: []string{TERMINAL_CMD, "--", "echo", "hello"},
		},
		{
			name: "nohup",
			cfg:  pkg.ConfigMap{"BINARY_PATH": "echo", "BINARY_ARGS": []any{"hello"}, "nohup": true},
			want: []string{"nohup", "echo", "hello"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := getCmd(tc.cfg)
			if !reflect.DeepEqual(cmd.Args, tc.want) {
				t.Fatalf("Args = %#v, want %#v", cmd.Args, tc.want)
			}
			if tc.dir != "" && cmd.Dir != tc.dir {
				t.Fatalf("Dir = %q, want %q", cmd.Dir, tc.dir)
			}
		})
	}
}

func TestGetPid(t *testing.T) {
	originalProcRoot := procRoot
	t.Cleanup(func() { procRoot = originalProcRoot })

	procRoot = t.TempDir()
	pidDir := filepath.Join(procRoot, "1234")
	if err := os.Mkdir(pidDir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "cmdline"), []byte("mybinary\x00--flag"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := getPid("mybinary")
	if err != nil {
		t.Fatalf("getPid() error = %v", err)
	}
	if got != 1234 {
		t.Fatalf("getPid() = %d, want 1234", got)
	}
	if got, err := getPid("missing"); err != nil || got != 0 {
		t.Fatalf("getPid(missing) = (%d, %v), want (0, nil)", got, err)
	}
}

func TestGetPidErrors(t *testing.T) {
	originalProcRoot := procRoot
	originalReadDir := osReadDir
	t.Cleanup(func() {
		procRoot = originalProcRoot
		osReadDir = originalReadDir
	})

	procRoot = t.TempDir()
	osReadDir = func(name string) ([]os.DirEntry, error) {
		return nil, errors.New("boom")
	}

	if _, err := getPid("anything"); err == nil {
		t.Fatal("getPid() error = nil, want error")
	}
}

func TestBinaryStatus(t *testing.T) {
	t.Run("oneshot", func(t *testing.T) {
		s := &BinaryStep{AbstractStep: &AbstractStep{config: pkg.ConfigMap{"oneshot": true}, state: StateRunning}}
		got, err := s.Status(context.Background())
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if got.Label != SymbolPlayButton || s.state != StateOff {
			t.Fatalf("Status() = %#v, state = %s", got, s.state)
		}
	})

	t.Run("pending", func(t *testing.T) {
		s := &BinaryStep{AbstractStep: &AbstractStep{config: pkg.ConfigMap{}, state: StatePending}, cmd: &exec.Cmd{}}
		got, err := s.Status(context.Background())
		if err != nil || got.Label != SymbolWorking {
			t.Fatalf("Status() = %#v, %v", got, err)
		}
	})

	t.Run("running", func(t *testing.T) {
		s := &BinaryStep{AbstractStep: &AbstractStep{config: pkg.ConfigMap{}, state: StateRunning}, cmd: &exec.Cmd{}}
		got, err := s.Status(context.Background())
		if err != nil || got.Label != SymbolCheckmark {
			t.Fatalf("Status() = %#v, %v", got, err)
		}
	})

	t.Run("paused", func(t *testing.T) {
		s := &BinaryStep{AbstractStep: &AbstractStep{config: pkg.ConfigMap{}, state: StatePaused}, cmd: &exec.Cmd{}}
		got, err := s.Status(context.Background())
		if err != nil || got.Label != SymbolPause {
			t.Fatalf("Status() = %#v, %v", got, err)
		}
	})

	t.Run("off", func(t *testing.T) {
		s := &BinaryStep{AbstractStep: &AbstractStep{config: pkg.ConfigMap{}, state: StateOff}, cmd: &exec.Cmd{}}
		got, err := s.Status(context.Background())
		if err != nil || got.Label != SymbolCross {
			t.Fatalf("Status() = %#v, %v", got, err)
		}
	})

	t.Run("attach", func(t *testing.T) {
		originalProcRoot := procRoot
		t.Cleanup(func() { procRoot = originalProcRoot })
		procRoot = t.TempDir()
		pidDir := filepath.Join(procRoot, "4242")
		if err := os.Mkdir(pidDir, 0o755); err != nil {
			t.Fatalf("Mkdir() error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(pidDir, "cmdline"), []byte("mybinary\x00--flag"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		s := &BinaryStep{AbstractStep: &AbstractStep{config: pkg.ConfigMap{"BINARY_PATH": "mybinary"}, state: StateOff}}
		got, err := s.Status(context.Background())
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if got.Label != SymbolCheckmark || s.state != StateRunning || s.cmd == nil {
			t.Fatalf("Status() = %#v, state = %s, cmd = %#v", got, s.state, s.cmd)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		s := &BinaryStep{AbstractStep: &AbstractStep{config: pkg.ConfigMap{}, state: StepState("invalid")}, cmd: &exec.Cmd{}}
		if _, err := s.Status(context.Background()); err == nil {
			t.Fatal("Status() error = nil, want error")
		}
	})
}

func TestBinaryActivateAndDeactivate(t *testing.T) {
	s := &BinaryStep{
		AbstractStep: &AbstractStep{
			label:  "sleep",
			config: pkg.ConfigMap{"BINARY_PATH": "sleep", "BINARY_ARGS": []any{"2"}, "oneshot": false},
			state:  StateOff,
		},
	}

	if err := s.Activate(context.Background()); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if s.state != StateRunning {
		t.Fatalf("Activate() state = %s, want running", s.state)
	}

	if err := s.Deactivate(context.Background()); err != nil {
		t.Fatalf("Deactivate() error = %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s.state == StateOff {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("Deactivate() did not transition state to off, got %s", s.state)
}
