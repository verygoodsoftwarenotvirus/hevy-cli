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

## 5/3/1

Each lift in the config stores a **training max** (`training_max_kg`); all working-set,
warmup, and assistance weights are derived from it. The two commands you'll run regularly:

```bash
hevy 531 sync               # (re)create all 16 routines (4 lifts × 4 weeks) at the current training maxes
hevy 531 sync --next-cycle  # advance a cycle: bump training maxes (upper +2.5kg, lower +5kg), then recreate routines in a fresh folder
```

Typical loop: train the 4-week block (deleting routines as you finish them), then run
`hevy 531 sync --next-cycle` to get a fresh block with heavier weights.

### Assistance presets

The supplemental volume after the main lift is a preset (`assistance` in the config),
switchable at any time — the next `hevy 531 sync` rewrites the routines to match:

```bash
hevy 531 assistance          # show the current preset
hevy 531 assistance fsl      # First Set Last: 5×5 at the week's first working-set weight
hevy 531 assistance bbb      # Boring But Big: 5×10 at 50% of the training max
hevy 531 assistance none     # main lift only
```

FSL sets are appended to the main lift's own exercise; BBB logs against the separate
`bbb_exercise_template_id` exercise. No preset adds volume on the deload week. A config
with no `assistance` key means BBB, which is what these configs did before presets existed.

## Development

```bash
make setup          # Install dev tools + vendor deps
make format         # Format all Go code
make lint           # Run golangci-lint + shellcheck
make test           # Run tests with race detector
make build          # Build all packages
```
