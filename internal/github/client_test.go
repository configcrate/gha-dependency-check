package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/configcrate/gha-dependency-check/internal/model"
)

func TestCheckHealthyDependency(t *testing.T) {
	t.Setenv("GH_TOKEN", "test-token")
	var sawAuthorization bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") == "Bearer test-token" {
			sawAuthorization = true
		}
		switch request.URL.Path {
		case "/repos/acme/action":
			fmt.Fprint(writer, `{"archived":false,"disabled":false}`)
		case "/repos/acme/action/commits/v1":
			fmt.Fprint(writer, `{"sha":"abc"}`)
		case "/repos/acme/action/contents/sub/action":
			if request.URL.Query().Get("ref") != "v1" {
				t.Errorf("contents ref = %q, want v1", request.URL.Query().Get("ref"))
			}
			fmt.Fprint(writer, `{"type":"dir"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, time.Second)
	checked := client.Check(context.Background(), model.Dependency{
		Owner: "acme",
		Repo:  "action",
		Path:  "sub/action",
		Ref:   "v1",
	})
	if checked.Status != model.StatusHealthy {
		t.Fatalf("Status = %s, want healthy (%s)", checked.Status, checked.Message)
	}
	if !sawAuthorization {
		t.Fatal("request did not include token from GH_TOKEN")
	}
}

func TestCheckRepositoryStates(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		headers    map[string]string
		want       model.Status
	}{
		{
			name:       "blocked",
			statusCode: http.StatusForbidden,
			body:       `{"message":"Repository access blocked"}`,
			want:       model.StatusBlocked,
		},
		{
			name:       "unavailable",
			statusCode: http.StatusNotFound,
			body:       `{"message":"Not Found"}`,
			want:       model.StatusUnavailable,
		},
		{
			name:       "rate limited",
			statusCode: http.StatusForbidden,
			body:       `{"message":"API rate limit exceeded"}`,
			headers:    map[string]string{"X-RateLimit-Remaining": "0"},
			want:       model.StatusAPIError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				for key, value := range test.headers {
					writer.Header().Set(key, value)
				}
				writer.WriteHeader(test.statusCode)
				fmt.Fprint(writer, test.body)
			}))
			defer server.Close()

			client := NewClient(server.URL, time.Second)
			checked := client.Check(context.Background(), model.Dependency{Owner: "acme", Repo: "action", Ref: "v1"})
			if checked.Status != test.want {
				t.Fatalf("Status = %s, want %s (%s)", checked.Status, test.want, checked.Message)
			}
		})
	}
}

func TestCheckArchivedAndDisabled(t *testing.T) {
	tests := []struct {
		name string
		body string
		want model.Status
	}{
		{name: "archived", body: `{"archived":true,"disabled":false}`, want: model.StatusArchived},
		{name: "disabled", body: `{"archived":false,"disabled":true}`, want: model.StatusDisabled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(writer, test.body)
			}))
			defer server.Close()

			checked := NewClient(server.URL, time.Second).Check(
				context.Background(),
				model.Dependency{Owner: "acme", Repo: "action", Ref: "v1"},
			)
			if checked.Status != test.want {
				t.Fatalf("Status = %s, want %s", checked.Status, test.want)
			}
		})
	}
}

func TestCheckMissingRefAndPath(t *testing.T) {
	t.Run("missing ref", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path == "/repos/acme/action" {
				fmt.Fprint(writer, `{"archived":false,"disabled":false}`)
				return
			}
			http.NotFound(writer, request)
		}))
		defer server.Close()

		checked := NewClient(server.URL, time.Second).Check(
			context.Background(),
			model.Dependency{Owner: "acme", Repo: "action", Ref: "missing"},
		)
		if checked.Status != model.StatusMissingRef {
			t.Fatalf("Status = %s, want missing_ref", checked.Status)
		}
	})

	t.Run("missing path", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/repos/acme/action":
				fmt.Fprint(writer, `{"archived":false,"disabled":false}`)
			case "/repos/acme/action/commits/v1":
				fmt.Fprint(writer, `{"sha":"abc"}`)
			default:
				http.NotFound(writer, request)
			}
		}))
		defer server.Close()

		checked := NewClient(server.URL, time.Second).Check(
			context.Background(),
			model.Dependency{Owner: "acme", Repo: "action", Path: "missing", Ref: "v1"},
		)
		if checked.Status != model.StatusMissingPath {
			t.Fatalf("Status = %s, want missing_path", checked.Status)
		}
	})
}
