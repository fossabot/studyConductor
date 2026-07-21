package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"studyConductor/pkg"
	"studyConductor/step"
)

type fakeStep struct {
	label            string
	state            step.StepState
	status           *step.Status
	statusErr        error
	activateCalled   chan struct{}
	deactivateCalled chan struct{}
}

func (f *fakeStep) Activate(ctx context.Context) error {
	if f.activateCalled != nil {
		select {
		case f.activateCalled <- struct{}{}:
		default:
		}
	}
	return nil
}

func (f *fakeStep) Deactivate(ctx context.Context) error {
	if f.deactivateCalled != nil {
		select {
		case f.deactivateCalled <- struct{}{}:
		default:
		}
	}
	return nil
}

func (f *fakeStep) Status(ctx context.Context) (*step.Status, error) {
	return f.status, f.statusErr
}

func (f *fakeStep) Label() string {
	return f.label
}

func (f *fakeStep) State() step.StepState {
	return f.state
}

type fakeTeaProgram struct{}

func (fakeTeaProgram) Run() (tea.Model, error) {
	return nil, nil
}

func TestToggleGesture(t *testing.T) {
	originalURL := gestureURL
	originalRequester := gestureRequester
	t.Cleanup(func() { gestureURL = originalURL })
	t.Cleanup(func() { gestureRequester = originalRequester })

	var body []byte
	gestureRequester = func(req *http.Request) (*http.Response, error) {
		defer req.Body.Close()
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		return nil, nil
	}

	model := &StudyModel{}
	if err := model.ToggleGesture(); err != nil {
		t.Fatalf("ToggleGesture() error = %v", err)
	}
	if !model.gestureDetected {
		t.Fatal("ToggleGesture() did not flip gestureDetected")
	}
	if !bytes.Equal(body, []byte(`{"g":true}`)) {
		t.Fatalf("ToggleGesture() body = %s, want %s", body, `{"g":true}`)
	}
}

func TestToggleGestureError(t *testing.T) {
	originalURL := gestureURL
	originalRequester := gestureRequester
	t.Cleanup(func() { gestureURL = originalURL })
	t.Cleanup(func() { gestureRequester = originalRequester })

	gestureURL = "http://%"
	model := &StudyModel{}
	if err := model.ToggleGesture(); err == nil {
		t.Fatal("ToggleGesture() error = nil, want error")
	}
}

func TestUpdateAndView(t *testing.T) {
	originalRequester := gestureRequester
	t.Cleanup(func() { gestureRequester = originalRequester })
	gestureRequester = func(req *http.Request) (*http.Response, error) {
		_, _ = io.ReadAll(req.Body)
		return nil, nil
	}

	activateCh := make(chan struct{}, 1)
	deactivateCh := make(chan struct{}, 1)
	model := &StudyModel{
		Context: context.Background(),
		steps: []step.Step{
			&fakeStep{label: "one", state: step.StateOff, status: &step.Status{Label: step.SymbolCross}, activateCalled: activateCh},
			&fakeStep{label: "two", state: step.StateRunning, status: &step.Status{Label: step.SymbolCheckmark}, deactivateCalled: deactivateCh},
		},
		selected: map[int]struct{}{},
	}

	if _, cmd := model.Update(tea.KeyMsg{Type: tea.KeyDown}); cmd != nil || model.cursor != 1 {
		t.Fatalf("Update(down) cursor=%d cmd=%v", model.cursor, cmd)
	}
	if _, cmd := model.Update(tea.KeyMsg{Type: tea.KeyDown}); cmd != nil || model.cursor != 1 {
		t.Fatalf("Update(boundary down) cursor=%d cmd=%v", model.cursor, cmd)
	}
	if _, cmd := model.Update(tea.KeyMsg{Type: tea.KeyUp}); cmd != nil || model.cursor != 0 {
		t.Fatalf("Update(up) cursor=%d cmd=%v", model.cursor, cmd)
	}
	if _, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}); cmd != nil || !model.gestureDetected {
		t.Fatalf("Update(g) gesture=%v cmd=%v", model.gestureDetected, cmd)
	}

	if _, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Fatalf("Update(enter) cmd=%v", cmd)
	}
	select {
	case <-activateCh:
	case <-time.After(time.Second):
		t.Fatal("Activate() was not called")
	}
	if _, ok := model.selected[0]; !ok {
		t.Fatal("selected map missing activated step")
	}

	model.cursor = 1
	if _, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Fatalf("Update(enter deactivate) cmd=%v", cmd)
	}
	select {
	case <-deactivateCh:
	case <-time.After(time.Second):
		t.Fatal("Deactivate() was not called")
	}

	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe() error = %v", err)
	}
	originalStderr := os.Stderr
	os.Stderr = stderrWriter
	defer func() {
		os.Stderr = originalStderr
		_ = stderrWriter.Close()
	}()

	model.steps = []step.Step{
		&fakeStep{label: "one", status: &step.Status{Label: step.SymbolCross}, statusErr: nil},
		&fakeStep{label: "two", status: &step.Status{Label: step.SymbolCheckmark}, statusErr: io.EOF},
	}
	view := model.View()
	_ = stderrWriter.Close()
	stderrBytes, _ := io.ReadAll(stderrReader)

	if !strings.Contains(view, "⇨ [") || !strings.Contains(view, "Gesture:") || !strings.Contains(view, "[Active]") {
		t.Fatalf("View() output missing expected fragments: %q", view)
	}
	if !strings.Contains(string(stderrBytes), "EOF") {
		t.Fatalf("View() stderr = %q, want EOF", stderrBytes)
	}
}

func TestRunAndMain(t *testing.T) {
	originalLoadDotenv := loadDotenv
	originalLoadConfig := loadConfig
	originalGetStorage := getStorage
	originalNewTeaProgram := newTeaProgram
	originalGestureURL := gestureURL
	originalGestureRequester := gestureRequester
	t.Cleanup(func() {
		loadDotenv = originalLoadDotenv
		loadConfig = originalLoadConfig
		getStorage = originalGetStorage
		newTeaProgram = originalNewTeaProgram
		gestureURL = originalGestureURL
		gestureRequester = originalGestureRequester
	})

	loadDotenv = func(filenames ...string) error { return nil }
	loadConfig = func() (*pkg.Config, error) {
		return &pkg.Config{
			Study: pkg.Study{Storage: pkg.ConfigMap{"local": "true", "path": "data"}},
			Modules: []pkg.Module{
				{Name: "bin", Type: pkg.ModuleTypeBinary, Configuration: pkg.ConfigMap{"BINARY_PATH": "/bin/true"}},
			},
		}, nil
	}
	getStorage = func(cfg *pkg.Config) (string, error) {
		return "/tmp/study", nil
	}
	newTeaProgram = func(model tea.Model) teaRunner {
		return fakeTeaProgram{}
	}
	gestureRequester = func(req *http.Request) (*http.Response, error) {
		_, _ = io.ReadAll(req.Body)
		return nil, nil
	}

	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	// Ensure main also routes through the same dependency seams.
	main()
}
