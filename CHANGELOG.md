# Changelog

All notable changes to this project will be documented here.

## [0.1.0] - 2026-07-03

### Added

- Scan repository workflow directories, custom directories, and individual YAML files.
- Detect remote GitHub Action and reusable workflow dependencies.
- Check repository, ref, and optional subpath availability through the GitHub API.
- Report blocked, unavailable, disabled, archived, missing, and invalid dependencies.
- Provide text and JSON output.
- Return separate exit codes for findings and operational errors.
