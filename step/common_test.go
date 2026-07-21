package step

import (
	"image/color"
	"reflect"
	"studyConductor/pkg"
	"testing"
)

func TestAbstractStepString(t *testing.T) {
	cases := []struct {
		name  string
		state StepState
		want  string
	}{
		{name: "running", state: StateRunning, want: "Stop demo"},
		{name: "pending", state: StatePending, want: "Wait... demo"},
		{name: "off", state: StateOff, want: "Start demo"},
		{name: "default", state: StepState("unknown"), want: "Start demo"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &AbstractStep{label: "demo", state: tc.state}
			if got := s.String(); got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildStep(t *testing.T) {
	binary, err := BuildStep(&pkg.Module{Name: "bin", Type: pkg.ModuleTypeBinary, Configuration: pkg.ConfigMap{"BINARY_PATH": "/bin/true"}})
	if err != nil {
		t.Fatalf("BuildStep(binary) error = %v", err)
	}
	if _, ok := binary.(*BinaryStep); !ok {
		t.Fatalf("BuildStep(binary) = %T, want *BinaryStep", binary)
	}

	docker, err := BuildStep(&pkg.Module{Name: "dock", Type: pkg.ModuleTypeDocker, Configuration: pkg.ConfigMap{"CONTAINER_NAME": "dock"}})
	if err != nil {
		t.Fatalf("BuildStep(docker) error = %v", err)
	}
	if _, ok := docker.(*ContainerStep); !ok {
		t.Fatalf("BuildStep(docker) = %T, want *ContainerStep", docker)
	}

	if _, err := BuildStep(&pkg.Module{Name: "bad", Type: pkg.ModuleType("unknown")}); err == nil {
		t.Fatal("BuildStep(unknown) error = nil, want error")
	}
}

func TestStatusColorStructsAreStable(t *testing.T) {
	got := Status{Label: SymbolCheckmark, Color: color.RGBA{G: 255}}
	if !reflect.DeepEqual(got, Status{Label: "✓", Color: color.RGBA{G: 255}}) {
		t.Fatalf("Status mismatch: %#v", got)
	}
}
