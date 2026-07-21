package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/specgen"
)

type fakePodmanResponse struct {
	process func(any) error
}

func (f fakePodmanResponse) Process(v interface{}) error {
	if f.process != nil {
		return f.process(v)
	}
	return nil
}

type fakePodmanClient struct {
	doRequest func(ctx context.Context, httpBody io.Reader, httpMethod, endpoint string, queryParams url.Values, headers http.Header, pathValues ...string) (podmanResponse, error)
}

func (f fakePodmanClient) DoRequest(ctx context.Context, httpBody io.Reader, httpMethod, endpoint string, queryParams url.Values, headers http.Header, pathValues ...string) (podmanResponse, error) {
	return f.doRequest(ctx, httpBody, httpMethod, endpoint, queryParams, headers, pathValues...)
}

func TestCreateWithSpec(t *testing.T) {
	originalGetPodmanClient := getPodmanClient
	t.Cleanup(func() { getPodmanClient = originalGetPodmanClient })

	var gotMethod, gotEndpoint string
	var gotBody []byte
	getPodmanClient = func(ctx context.Context) (podmanConnection, error) {
		return fakePodmanClient{
			doRequest: func(ctx context.Context, httpBody io.Reader, httpMethod, endpoint string, queryParams url.Values, headers http.Header, pathValues ...string) (podmanResponse, error) {
				gotMethod = httpMethod
				gotEndpoint = endpoint
				gotBody, _ = io.ReadAll(httpBody)
				return fakePodmanResponse{
					process: func(v any) error {
						resp := v.(*entities.ContainerCreateResponse)
						resp.ID = "abc"
						return nil
					},
				}, nil
			},
		}, nil
	}

	sg := specgen.NewSpecGenerator("alpine", false)
	sg.Name = "demo"
	resp, err := CreateWithSpec(context.Background(), sg)
	if err != nil {
		t.Fatalf("CreateWithSpec() error = %v", err)
	}
	if resp.ID != "abc" {
		t.Fatalf("CreateWithSpec() response ID = %q, want abc", resp.ID)
	}
	if gotMethod != http.MethodPost || gotEndpoint != "/containers/create" {
		t.Fatalf("CreateWithSpec() method/endpoint = %s %s", gotMethod, gotEndpoint)
	}
	if !strings.Contains(string(gotBody), `"name":"demo"`) {
		t.Fatalf("CreateWithSpec() body = %s, want container name", gotBody)
	}
}

func TestList(t *testing.T) {
	originalGetPodmanClient := getPodmanClient
	t.Cleanup(func() { getPodmanClient = originalGetPodmanClient })

	var gotQuery url.Values
	getPodmanClient = func(ctx context.Context) (podmanConnection, error) {
		return fakePodmanClient{
			doRequest: func(ctx context.Context, httpBody io.Reader, httpMethod, endpoint string, queryParams url.Values, headers http.Header, pathValues ...string) (podmanResponse, error) {
				gotQuery = queryParams
				return fakePodmanResponse{
					process: func(v any) error {
						resp := v.(*[]entities.ListContainer)
						*resp = []entities.ListContainer{{ID: "abc"}}
						return nil
					},
				}, nil
			},
		}, nil
	}

	got, err := List(context.Background(), "demo")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "abc" {
		t.Fatalf("List() = %#v, want one container", got)
	}
	if gotQuery.Get("limit") != "1" {
		t.Fatalf("List() limit = %q, want 1", gotQuery.Get("limit"))
	}
	var filters map[string][]string
	if err := json.Unmarshal([]byte(gotQuery.Get("filters")), &filters); err != nil {
		t.Fatalf("List() filters JSON invalid: %v", err)
	}
	if !reflect.DeepEqual(filters["name"], []string{"demo"}) {
		t.Fatalf("List() filters = %#v, want name filter", filters)
	}
}

func TestStart(t *testing.T) {
	originalGetPodmanClient := getPodmanClient
	t.Cleanup(func() { getPodmanClient = originalGetPodmanClient })

	var gotEndpoint string
	var gotPathValues []string
	getPodmanClient = func(ctx context.Context) (podmanConnection, error) {
		return fakePodmanClient{
			doRequest: func(ctx context.Context, httpBody io.Reader, httpMethod, endpoint string, queryParams url.Values, headers http.Header, pathValues ...string) (podmanResponse, error) {
				gotEndpoint = endpoint
				gotPathValues = append([]string{}, pathValues...)
				return fakePodmanResponse{}, nil
			},
		}, nil
	}

	if err := Start(context.Background(), "abc"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if gotEndpoint != "/containers/%s/start" {
		t.Fatalf("Start() endpoint = %q, want %q", gotEndpoint, "/containers/%s/start")
	}
	if !reflect.DeepEqual(gotPathValues, []string{"abc"}) {
		t.Fatalf("Start() pathValues = %#v, want %#v", gotPathValues, []string{"abc"})
	}
}

func TestGetPodmanClientError(t *testing.T) {
	originalGetPodmanClient := getPodmanClient
	t.Cleanup(func() { getPodmanClient = originalGetPodmanClient })

	getPodmanClient = func(ctx context.Context) (podmanConnection, error) {
		return nil, errors.New("boom")
	}

	if _, err := CreateWithSpec(context.Background(), specgen.NewSpecGenerator("alpine", false)); err == nil {
		t.Fatal("CreateWithSpec() error = nil, want error")
	}
}
