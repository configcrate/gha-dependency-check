# Security Policy

## Reporting a vulnerability

Please report vulnerabilities through GitHub private vulnerability reporting for this
repository. Do not include access tokens, private workflow contents, or other secrets
in a public issue.

## Data handling

`gha-dependency-check` reads workflow files from the selected local path and sends
read-only requests to the configured GitHub API. It does not upload workflow content,
execute actions, or write to repositories.

Authentication is read from `GH_TOKEN` or `GITHUB_TOKEN`. Tokens are sent only in the
GitHub API `Authorization` header and are never included in normal output.

Official release binaries are built with the current patched Go 1.25 toolchain and
checked with `govulncheck` before packaging.
