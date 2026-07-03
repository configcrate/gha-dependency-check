# Article Handoff: gha-dependency-check

Status: implementation complete, pending public release

## Article angle

Primary title:

```text
How to Find Blocked GitHub Actions Before Your Workflow Fails
```

Alternative:

```text
Check GitHub Action Dependencies for "Repository Access Blocked"
```

The article should lead with the user problem, not with the tool name.

## Exact problem

A workflow may keep a valid-looking line such as:

```yaml
- uses: actions-cool/issues-helper@v3
```

The referenced repository currently returns:

```text
HTTP 403
Repository access blocked
```

This is different from a normal `GITHUB_TOKEN` permissions failure. GitHub has made
the action repository unavailable, so changing workflow token permissions does not
repair the dependency.

## Current demand evidence

Snapshot taken 2026-07-03:

- 116 GitHub issues created in 2026 contained the exact phrase
  `Repository access blocked`.
- About 2,084 GitHub code results referenced `actions-cool/issues-helper`.
- About 441 referenced `actions-cool/maintain-one-comment`.
- Both action repository API endpoints returned HTTP 403 with
  `Repository access blocked`.

Counts will change. If the article is published later, describe them as a dated
snapshot or refresh them.

## What the tool does

`gha-dependency-check`:

1. Parses GitHub Actions workflow YAML.
2. Finds static remote `uses:` values.
3. Checks repository availability through the GitHub REST API.
4. Checks the referenced commit, branch, or tag.
5. Checks an optional action subdirectory or reusable workflow path.
6. Reports findings in text or JSON.
7. Returns CI-friendly exit codes.

It only sends GET requests and never executes action code.

## Verified commands

```bash
go install github.com/configcrate/gha-dependency-check/cmd/gha-dependency-check@latest
gha-dependency-check .
gha-dependency-check --format json . > gha-dependencies.json
```

Authenticated use:

```bash
export GH_TOKEN="$(gh auth token)"
gha-dependency-check .
```

PowerShell:

```powershell
$env:GH_TOKEN = gh auth token
gha-dependency-check .
```

## Exit codes

- `0`: all checked dependencies are healthy
- `1`: a dependency finding exists
- `2`: parsing, network, rate-limit, or API failure prevented a reliable check

## Honest limitations

- A 404 does not prove deletion; the token may lack access to a private repository.
- Dynamic matrix expressions cannot be checked statically.
- Local and Docker actions are skipped.
- Archived repositories are warnings/failures by design.
- This checks availability, not security quality or SHA pinning.
- GitHub API limits apply.

## Comparison

- `actionlint`: syntax and semantic linting; does not focus on live dependency
  repository health.
- `zizmor` and `octoscan`: security analysis.
- `gh-actions-doctor`: broad static workflow quality checks.
- `gha-dependency-check`: narrow online availability, ref, and path resolution.

These tools are complementary.

## Sources

- https://api.github.com/repos/actions-cool/issues-helper
- https://api.github.com/repos/actions-cool/maintain-one-comment
- https://github.com/search?q=%22Repository+access+blocked%22&type=issues
- https://github.com/search?q=%22actions-cool%2Fissues-helper%22&type=code
- https://github.com/search?q=%22actions-cool%2Fmaintain-one-comment%22&type=code
- https://docs.github.com/en/rest/repos/repos#get-a-repository
- https://docs.github.com/en/rest/commits/commits#get-a-commit
- https://docs.github.com/en/rest/repos/contents#get-repository-content

## Writing constraints

- Do not invent a personal story or claim the author personally experienced the
  outage.
- Keep the distinction between a blocked action repository and workflow token
  permission errors precise.
- Refresh live counts before publication.
- Include the repository and release URLs after publication.
