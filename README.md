# Lintme

A convenience wrapper around `golangci-lint` that discovers Go modules in your
workspace and runs the linter per module with zero configuration fuss.

## Overview

In a Go workspace (`go.work`) with many modules, running `golangci-lint`
requires `cd`-ing into each module directory and invoking it separately.
`lintme` automates that loop — one command from anywhere in the workspace lints
everything.

Module discovery works with workspaces: when `lintme` finds a `go.work` file,
it lints every module listed under a `use` directive, in order. When no
`go.work` exists, it walks up from the current directory until it finds a
`go.mod` and lints that single module.

For each module, `lintme` also walks up the directory tree to find the nearest
`.golangci.yaml` (or an equivalent config file), so each module can carry its
own linter config. Output streams in real time — nothing buffers it.

## Requirements

- [`golangci-lint`](https://golangci-lint.run/usage/install/) installed and available on `PATH`
- Go 1.26 or later

## Installation

```sh
go install github.com/StevenACoffman/lintme@latest
```

This places the `lintme` binary in `$GOPATH/bin` or `$GOBIN`. Ensure that
directory is on your `PATH`.

## Commands

Running bare `lintme` defaults to `lintme branch` — it lints only issues
introduced on the current branch since its merge-base with the remote default
branch.

### `lintme branch` (Default)

Lint only issues introduced on the current branch.

```sh
# Lint issues introduced on the current branch (auto-detects merge-base)
lintme

# Specify the base branch explicitly
lintme branch -B main

# Apply formatting instead of linting
lintme --fmt-only

# Forward extra flags to every golangci-lint invocation
lintme -- --timeout=5m
```

`lintme branch` queries `git ls-remote --symref origin HEAD` to determine the
remote default branch without requiring a local checkout. Use `-B` when there
is no remote or the remote is not named `origin`.

| Flag           | Default | Description                                                                      |
| -------------- | ------- | -------------------------------------------------------------------------------- |
| `-B`, `--base` | —       | Base branch for the merge-base computation; default: remote HEAD via `ls-remote` |

### `lintme run`

Lint all modules in the workspace, regardless of branch.

```sh
# Lint all modules, applying --fix (default)
lintme run

# Check only — do not modify files
lintme run --no-fix

# Format using golangci-lint fmt instead of golangci-lint run
lintme run --fmt-only

# Report only issues introduced since a given commit
lintme run --new-from-rev=main

# Forward extra flags to every golangci-lint invocation
lintme run -- --timeout=5m
lintme run --no-fix -- --timeout=5m --out-format=json
```

### `lintme pr`

Lint only issues introduced by a GitHub pull request.

```sh
# Lint the PR for the current branch (auto-detects PR number)
lintme pr

# Lint a specific PR
lintme pr 123

# With an explicit token and repo
lintme pr --token=$GITHUB_TOKEN --repo=owner/repo 123
```

When `<pr-number>` is omitted, `lintme pr` detects the open PR for the current
branch via the GitHub API, matching the behavior of `gh pr view`.

| Flag           | Default                    | Description                               |
| -------------- | -------------------------- | ----------------------------------------- |
| `--token`      | `$GITHUB_TOKEN`            | GitHub personal access token              |
| `--repo`       | detected from `git remote` | Repository as `owner/repo`                |
| `--github-url` | `$GITHUB_API_URL`          | GitHub API base URL for GitHub Enterprise |

### `lintme version`

Print build and version information.

```sh
lintme version         # human-readable table
lintme version --json  # machine-readable JSON
```

## Shared Flags

These flags work with all commands that invoke `golangci-lint`.

| Flag                   | Default | Description                                                                                                               |
| ---------------------- | ------- | ------------------------------------------------------------------------------------------------------------------------- |
| `--no-fix`             | off     | Skip `--fix`; report issues without modifying files                                                                       |
| `--fmt-only`           | off     | Run `golangci-lint fmt` instead of `golangci-lint run`; mutually exclusive with `--no-fix`                                |
| `--new-from-rev=<rev>` | —       | Pass `--new-from-rev=<rev>` to every golangci-lint invocation; golangci-lint reports only issues introduced since `<rev>` |

## Module and Config Discovery

### Module Discovery

`lintme` walks up from the current directory looking for a `go.work` file. When
it finds one, it lints every module listed under a `use` directive in
declaration order. When no `go.work` exists, it continues walking up until it
finds a `go.mod` and lints that single module.

### Config Discovery

For each module, `lintme` walks up from the module directory looking for a
config file in this priority order:

1. `.golangci.yml`
2. `.golangci.yaml`
3. `.golangci.toml`
4. `.golangci.json`

The output header for each module shows the resolved path. When no config file
exists, `golangci-lint` runs without `--config` and applies its own defaults.

## Environment Variables

Every flag can also be set via a `LINTME_`-prefixed environment variable. The
mapping rule: prepend `LINTME_`, uppercase, replace dashes with underscores.

| Flag             | Environment variable        |
| ---------------- | --------------------------- |
| `--no-fix`       | `LINTME_NO_FIX=true`        |
| `--fmt-only`     | `LINTME_FMT_ONLY=true`      |
| `--new-from-rev` | `LINTME_NEW_FROM_REV=<rev>` |

Flags on the command line always take precedence over environment variables.

## Output

```text
==> ./services/auth (github.com/example/myapp/services/auth)  config: ./services/auth/.golangci.yaml
... golangci-lint output ...

==> ./services/payments (github.com/example/myapp/services/payments)  config: no config
... golangci-lint output ...

2/2 modules passed
```

When a module fails:

```text
==> ./services/payments (github.com/example/myapp/services/payments)  config: no config
./services/payments/handler.go:42:9: unused variable (deadcode)
FAIL  ./services/payments: golangci-lint: exit status 1

1/2 modules passed
```

Failing modules do not stop remaining modules from linting. `lintme` exits
non-zero if any module fails.

## Exit Codes

| Code | Meaning                    |
| ---- | -------------------------- |
| 0    | All modules passed         |
| 1    | One or more modules failed |

## CI Integration

Use `--no-fix` in CI so the linter reports issues without modifying files.

```yaml
- name: Lint
  run: lintme --no-fix
```

## License

See [LICENSE](./LICENSE).
