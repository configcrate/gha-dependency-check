# gha-dependency-check

Find unavailable GitHub Actions dependencies before a workflow fails.

`gha-dependency-check` scans remote `uses:` entries and asks the GitHub API whether
the repository, ref, and optional action path still exist. It detects:

- repositories blocked by GitHub with `Repository access blocked`;
- deleted or inaccessible repositories;
- disabled repositories;
- archived repositories;
- missing tags, branches, or commit SHAs;
- missing action directories and reusable workflow files;
- malformed remote `uses:` values.

The checker is read-only. It never executes a workflow or downloads action code.

## Quick start

Install with Go:

```bash
go install github.com/configcrate/gha-dependency-check/cmd/gha-dependency-check@latest
```

Then run it at the root of a repository:

```bash
gha-dependency-check .
```

You can also scan one workflow file or a directory:

```bash
gha-dependency-check .github/workflows/release.yml
gha-dependency-check .github/workflows
```

## Example

```text
gha-dependency-check: scanned 2 workflow file(s), found 3 remote dependency reference(s)
HEALTHY      actions/checkout@v7  .github/workflows/ci.yml:12  repository, ref, and path are available
BLOCKED      actions-cool/issues-helper@v3  .github/workflows/issues.yml:18  Repository access blocked
HEALTHY      actions/setup-go@v6  .github/workflows/ci.yml:13  repository, ref, and path are available
summary: 2 healthy, 1 finding(s), 0 API error(s)
```

## Authentication and rate limits

Public repositories can be checked without authentication, subject to GitHub's low
anonymous API rate limit. For normal use and private action repositories, set either
`GH_TOKEN` or `GITHUB_TOKEN`.

With GitHub CLI:

```bash
export GH_TOKEN="$(gh auth token)"
gha-dependency-check .
```

PowerShell:

```powershell
$env:GH_TOKEN = gh auth token
gha-dependency-check .
```

The token only needs read access to the repositories being checked. Tokens are never
printed.

## JSON output

```bash
gha-dependency-check --format json . > gha-dependencies.json
```

Each result includes its workflow file, line, `uses:` value, status, HTTP status,
and a short diagnosis.

## Exit codes

| Code | Meaning |
| ---: | --- |
| `0` | Every checked dependency is healthy |
| `1` | At least one blocked, unavailable, disabled, archived, missing, or invalid dependency was found |
| `2` | The workflow could not be parsed or a GitHub API/network error prevented a reliable result |

## CI usage

```yaml
name: Check action dependencies

on:
  pull_request:
    paths:
      - ".github/workflows/**"
  schedule:
    - cron: "17 6 * * 1"

permissions:
  contents: read

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v6
        with:
          go-version: "1.25.x"
      - run: go run github.com/configcrate/gha-dependency-check/cmd/gha-dependency-check@latest .
        env:
          GH_TOKEN: ${{ github.token }}
```

A weekly schedule catches repositories that become unavailable even when your own
workflow files have not changed.

## CLI reference

```text
Usage: gha-dependency-check [options] [path]

  -api-url string
        GitHub API base URL (defaults to GITHUB_API_URL or api.github.com)
  -format string
        output format: text or json (default "text")
  -timeout duration
        timeout for each GitHub API request (default 10s)
  -version
        print version and exit
```

`GITHUB_API_URL` and `--api-url` make the checker usable with GitHub Enterprise
Server, provided its REST API implements the repository, commit, and contents
endpoints used by the checker.

## Scope and limitations

- A `404` can mean a repository was deleted, made private, or is hidden from the
  supplied token. The checker reports this as unavailable rather than guessing.
- Dynamic `uses:` expressions such as `${{ matrix.action }}` cannot be resolved
  statically and are skipped.
- Local actions (`./path`) and Docker actions (`docker://image`) are outside this
  tool's scope.
- An archived repository is reported as a finding even if its existing refs still
  work, because it is read-only and no longer maintained.
- The checker verifies availability, not workflow security. Use tools such as
  `actionlint`, `zizmor`, or `gh-actions-doctor` for complementary static checks.

## Development

```bash
go test ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
go build ./cmd/gha-dependency-check
```

Use a currently supported Go patch release when building binaries. Standard library
security fixes are delivered with the Go toolchain rather than through `go.mod`.

## License

MIT
