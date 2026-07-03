package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanFindsRemoteUsesAndSkipsLocalValues(t *testing.T) {
	root := t.TempDir()
	workflows := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflows, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `
name: CI
on: push
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: "acme/example/sub/action@feature/ref"
      - uses: ./local-action
      - uses: docker://alpine:3.20
      - uses: ${{ matrix.dynamic_action }}
      - uses: acme/missing-ref
`
	path := filepath.Join(workflows, "ci.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan returned an error: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("Files = %d, want 1", len(result.Files))
	}
	if len(result.Dependencies) != 2 {
		t.Fatalf("Dependencies = %d, want 2", len(result.Dependencies))
	}
	if len(result.Invalid) != 1 {
		t.Fatalf("Invalid = %d, want 1", len(result.Invalid))
	}

	dependency := result.Dependencies[1]
	if dependency.Owner != "acme" || dependency.Repo != "example" {
		t.Fatalf("parsed repository = %s/%s, want acme/example", dependency.Owner, dependency.Repo)
	}
	if dependency.Path != "sub/action" {
		t.Fatalf("Path = %q, want sub/action", dependency.Path)
	}
	if dependency.Ref != "feature/ref" {
		t.Fatalf("Ref = %q, want feature/ref", dependency.Ref)
	}
	if dependency.Line == 0 {
		t.Fatal("Line was not recorded")
	}
}

func TestScanAcceptsOneWorkflowFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "release.yaml")
	if err := os.WriteFile(path, []byte("jobs:\n  call:\n    uses: acme/reusable/.github/workflows/release.yml@v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Scan(path)
	if err != nil {
		t.Fatalf("Scan returned an error: %v", err)
	}
	if len(result.Dependencies) != 1 {
		t.Fatalf("Dependencies = %d, want 1", len(result.Dependencies))
	}
	if got := result.Dependencies[0].Path; got != ".github/workflows/release.yml" {
		t.Fatalf("Path = %q, want reusable workflow path", got)
	}
}

func TestScanReportsInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.yml")
	if err := os.WriteFile(path, []byte("jobs: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Scan(path)
	if err == nil {
		t.Fatal("Scan returned nil error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "parse workflow") {
		t.Fatalf("error = %q, want parse workflow context", err)
	}
}
