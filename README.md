# gopher

My template repo for Go projects.

## What's included

- **Makefile** with targets for formatting, linting, building, and testing
- **`.golangci.yml`** with 42+ linters configured (golangci-lint v2)
- **`scripts/`** directory with shell scripts for all build/format/lint/test operations
- **GitHub Actions** workflows for CI (build, formatting, linting, shellcheck, unit tests)
- **GitHub templates** for issues and pull requests

## Usage

1. Create a new repo from this template
2. Update `go.mod` module path
3. Update `THIS` in `Makefile` to match the new module path
4. Update `prefix()` sections in `.golangci.yml` formatters
5. Update module path in `scripts/test.sh`, `scripts/format_imports.sh`, and `scripts/build.sh`
6. Update `CLAUDE.md` with project-specific details
7. Run `make setup`

## Development

```bash
make setup          # Install dev tools + vendor deps
make format         # Format all Go code
make lint           # Run golangci-lint + shellcheck
make test           # Run tests with race detector
make build          # Build all packages
```
