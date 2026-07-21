package step

import (
	"context"
	"os"
	"reflect"
	"testing"

	"go.podman.io/common/libnetwork/types"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/bindings/containers"
	"go.podman.io/podman/v6/pkg/bindings/images"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/entities/reports"
	podmantypes "go.podman.io/podman/v6/pkg/domain/entities/types"
	"go.podman.io/podman/v6/pkg/specgen"
	"studyConductor/pkg"
)

func TestBoolPtrAndMatchName(t *testing.T) {
	if got := boolPtr(true); got == nil || !*got {
		t.Fatalf("boolPtr() = %v, want true pointer", got)
	}
	if !matchName([]string{"localhost/my-image:latest"}, "my-image:latest") {
		t.Fatal("matchName() = false, want true")
	}
	if matchName([]string{"localhost/other:latest"}, "my-image:latest") {
		t.Fatal("matchName() = true, want false")
	}
}

func TestCreatePortMappings(t *testing.T) {
	got, err := createPortMappings([]string{"8080:80", "", "9090:90"})
	if err != nil {
		t.Fatalf("createPortMappings() error = %v", err)
	}
	want := []types.PortMapping{
		{HostPort: 8080, ContainerPort: 80},
		{HostPort: 9090, ContainerPort: 90},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("createPortMappings() = %#v, want %#v", got, want)
	}

	if _, err := createPortMappings([]string{"8080"}); err == nil {
		t.Fatal("createPortMappings() invalid map error = nil, want error")
	}
	if _, err := createPortMappings([]string{"a:80"}); err == nil {
		t.Fatal("createPortMappings() invalid host error = nil, want error")
	}
}

func TestContainerActivateDeactivateAndStatus(t *testing.T) {
	originalNewConn := newPodmanConnection
	originalGetClient := getPodmanClient
	originalListImages := listImages
	originalPullImage := pullImage
	originalCreateContainer := createContainer
	originalStartContainer := startContainer
	originalStopContainer := stopContainer
	originalRemoveContainer := removeContainer
	originalListContainers := listContainers
	t.Cleanup(func() {
		newPodmanConnection = originalNewConn
		getPodmanClient = originalGetClient
		listImages = originalListImages
		pullImage = originalPullImage
		createContainer = originalCreateContainer
		startContainer = originalStartContainer
		stopContainer = originalStopContainer
		removeContainer = originalRemoveContainer
		listContainers = originalListContainers
	})

	newPodmanConnection = func(ctx context.Context, socket string) (context.Context, error) {
		return ctx, nil
	}
	getPodmanClient = func(ctx context.Context) (*bindings.Connection, error) {
		return nil, nil
	}

	t.Run("activate pulls and starts when image missing", func(t *testing.T) {
		var pulled bool
		var created bool
		var started string
		listImages = func(ctx context.Context, options *images.ListOptions) ([]*podmantypes.ImageSummary, error) {
			return []*podmantypes.ImageSummary{{Names: []string{"localhost/other:latest"}}}, nil
		}
		pullImage = func(ctx context.Context, rawImage string, options *images.PullOptions) ([]string, error) {
			pulled = true
			return nil, nil
		}
		createContainer = func(ctx context.Context, s *specgen.SpecGenerator) (entities.ContainerCreateResponse, error) {
			created = true
			return entities.ContainerCreateResponse{ID: "abc"}, nil
		}
		startContainer = func(ctx context.Context, nameOrID string) error {
			started = nameOrID
			return nil
		}
		listContainers = func(ctx context.Context, name string) ([]entities.ListContainer, error) {
			return nil, nil
		}

		s := &ContainerStep{
			AbstractStep: &AbstractStep{
				label:  "demo",
				config: pkg.ConfigMap{"IMAGE_NAME": "localhost/demo:latest", "CONTAINER_NAME": "demo", "ENV": pkg.ConfigMap{}, "DATA_PATH": ""},
				state:  StateOff,
			},
		}
		if err := s.Activate(context.Background()); err != nil {
			t.Fatalf("Activate() error = %v", err)
		}
		if !pulled || !created || started != "abc" {
			t.Fatalf("Activate() pulled=%v created=%v started=%q", pulled, created, started)
		}
	})

	t.Run("activate reuses existing local image and container", func(t *testing.T) {
		var pulled bool
		var started string
		listImages = func(ctx context.Context, options *images.ListOptions) ([]*podmantypes.ImageSummary, error) {
			return []*podmantypes.ImageSummary{{Names: []string{"localhost/demo:latest"}}}, nil
		}
		pullImage = func(ctx context.Context, rawImage string, options *images.PullOptions) ([]string, error) {
			pulled = true
			return nil, nil
		}
		createContainer = func(ctx context.Context, s *specgen.SpecGenerator) (entities.ContainerCreateResponse, error) {
			t.Fatal("createContainer should not be called when container exists")
			return entities.ContainerCreateResponse{}, nil
		}
		startContainer = func(ctx context.Context, nameOrID string) error {
			started = nameOrID
			return nil
		}
		listContainers = func(ctx context.Context, name string) ([]entities.ListContainer, error) {
			return []entities.ListContainer{{ID: "existing", Names: []string{"demo"}}}, nil
		}

		s := &ContainerStep{
			AbstractStep: &AbstractStep{
				label:  "demo",
				config: pkg.ConfigMap{"IMAGE_NAME": "localhost/demo:latest", "CONTAINER_NAME": "demo", "ENV": pkg.ConfigMap{}, "DATA_PATH": ""},
				state:  StateOff,
			},
		}
		if err := s.Activate(context.Background()); err != nil {
			t.Fatalf("Activate() error = %v", err)
		}
		if pulled {
			t.Fatal("pullImage called for local image")
		}
		if started != "existing" {
			t.Fatalf("startContainer called with %q, want existing", started)
		}
	})

	t.Run("deactivate", func(t *testing.T) {
		var stopped, removed string
		stopContainer = func(ctx context.Context, nameOrID string, options *containers.StopOptions) error {
			stopped = nameOrID
			return nil
		}
		removeContainer = func(ctx context.Context, nameOrID string, options *containers.RemoveOptions) ([]*reports.RmReport, error) {
			removed = nameOrID
			return nil, nil
		}
		listContainers = func(ctx context.Context, name string) ([]entities.ListContainer, error) {
			return []entities.ListContainer{{ID: "abc", Names: []string{"demo"}}}, nil
		}

		s := &ContainerStep{
			AbstractStep: &AbstractStep{
				label:  "demo",
				config: pkg.ConfigMap{"CONTAINER_NAME": "demo"},
				state:  StateRunning,
			},
		}
		if err := s.Deactivate(context.Background()); err != nil {
			t.Fatalf("Deactivate() error = %v", err)
		}
		if stopped != "abc" || removed != "abc" {
			t.Fatalf("Deactivate() stopped=%q removed=%q, want abc/abc", stopped, removed)
		}
	})

	t.Run("status", func(t *testing.T) {
		listContainers = func(ctx context.Context, name string) ([]entities.ListContainer, error) {
			return nil, nil
		}
		s := &ContainerStep{
			AbstractStep: &AbstractStep{
				label:  "demo",
				config: pkg.ConfigMap{"CONTAINER_NAME": "demo"},
			},
		}
		got, err := s.Status(context.Background())
		if err != nil || got.Label != SymbolCross || s.state != StateOff {
			t.Fatalf("Status(no container) = %#v, %v, state=%s", got, err, s.state)
		}

		s.workingMx.Store(true)
		got, err = s.Status(context.Background())
		if err != nil || got.Label != SymbolWorking || s.state != StatePending {
			t.Fatalf("Status(working) = %#v, %v, state=%s", got, err, s.state)
		}
		s.workingMx.Store(false)

		listContainers = func(ctx context.Context, name string) ([]entities.ListContainer, error) {
			return []entities.ListContainer{{ID: "abc", Names: []string{"demo"}, State: StateConfigured}}, nil
		}
		got, err = s.Status(context.Background())
		if err != nil || got.Label != SymbolPause || s.state != StateOff {
			t.Fatalf("Status(configured) = %#v, %v, state=%s", got, err, s.state)
		}

		listContainers = func(ctx context.Context, name string) ([]entities.ListContainer, error) {
			return []entities.ListContainer{{ID: "abc", Names: []string{"demo"}, State: "running"}}, nil
		}
		got, err = s.Status(context.Background())
		if err != nil || got.Label != SymbolCheckmark || s.state != StateRunning {
			t.Fatalf("Status(running) = %#v, %v, state=%s", got, err, s.state)
		}
	})
}

func TestHasLocalImageAndMounts(t *testing.T) {
	originalListImages := listImages
	originalCreateVolume := createVolume
	originalMkdirAll := mkdirAll
	originalConf := Conf
	t.Cleanup(func() {
		listImages = originalListImages
		createVolume = originalCreateVolume
		mkdirAll = originalMkdirAll
		Conf = originalConf
	})

	s := &ContainerStep{AbstractStep: &AbstractStep{config: pkg.ConfigMap{"IMAGE_NAME": "localhost/demo:latest"}}}

	listImages = func(ctx context.Context, options *images.ListOptions) ([]*podmantypes.ImageSummary, error) {
		return []*podmantypes.ImageSummary{{Names: []string{"registry.local/demo:latest"}}}, nil
	}
	got, err := s.hasLocalImage("demo:latest")
	if err != nil || !got {
		t.Fatalf("hasLocalImage() = %v, %v", got, err)
	}

	Conf = &pkg.Config{Study: pkg.Study{Storage: pkg.ConfigMap{"path": t.TempDir()}}}
	var mkdirPath, volumePath string
	mkdirAll = func(path string, perm os.FileMode) error {
		mkdirPath = path
		return nil
	}
	createVolume = func(name, path string) (string, error) {
		volumePath = path
		return name, nil
	}
	vols, err := s.createMounts("/data")
	if err != nil {
		t.Fatalf("createMounts() error = %v", err)
	}
	if mkdirPath == "" || volumePath == "" || len(vols) != 1 || vols[0].Name != "study_storage" {
		t.Fatalf("createMounts() unexpected result: mkdirPath=%q volumePath=%q vols=%#v", mkdirPath, volumePath, vols)
	}
}
