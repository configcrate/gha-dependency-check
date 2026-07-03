package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHealthyWorkflow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/actions/checkout":
			fmt.Fprint(writer, `{"archived":false,"disabled":false}`)
		case "/repos/actions/checkout/commits/v4":
			fmt.Fprint(writer, `{"sha":"abc"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "ci.yml")
	if err := os.WriteFile(path, []byte("steps:\n  - uses: actions/checkout@v4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(
		[]string{"--format", "json", "--api-url", server.URL, path},
		&stdout,
		&stderr,
	)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status": "healthy"`) {
		t.Fatalf("JSON output did not include healthy result: %s", stdout.String())
	}
}

func TestRunReturnsFindingExitCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
		fmt.Fprint(writer, `{"message":"Repository access blocked"}`)
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "ci.yml")
	if err := os.WriteFile(path, []byte("steps:\n  - uses: blocked/action@v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"--api-url", server.URL, path}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "BLOCKED") {
		t.Fatalf("text output did not include BLOCKED: %s", stdout.String())
	}
}

func TestRunCachesRepeatedDependencyChecks(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		switch request.URL.Path {
		case "/repos/actions/checkout":
			fmt.Fprint(writer, `{"archived":false,"disabled":false}`)
		case "/repos/actions/checkout/commits/v4":
			fmt.Fprint(writer, `{"sha":"abc"}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "ci.yml")
	content := "steps:\n  - uses: actions/checkout@v4\n  - uses: actions/checkout@v4\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"--api-url", server.URL, path}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", exitCode, stderr.String())
	}
	if requests != 2 {
		t.Fatalf("GitHub API requests = %d, want 2 for one unique dependency", requests)
	}
}

func TestSafeTextRemovesTerminalControls(t *testing.T) {
	got := safeText("normal\x1b[31m\n")
	if got != "normal?[31m?" {
		t.Fatalf("safeText() = %q, want control characters replaced", got)
	}
}
