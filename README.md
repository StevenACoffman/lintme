# lintme

A convenience wrapper around `golangci-lint` that discovers Go modules in your workspace and runs the linter per module with zero configuration fuss.

## Overview

In a Go workspace (`go.work`) with multiple modules, running `golangci-lint` requires `cd`-ing into each module directory and invoking it separately. `lintme` automates that loop — one command from anywhere in the workspace lints everything.

Module discovery is workspace-aware: if a `go.work` file is found, `lintme` lints every module listed under a `use` directive, in order. If no `go.work` is found, it walks up from the current directory until it finds a `go.mod` and lints that single module.

For each module, `lintme` also walks up the directory tree to find the nearest `.golangci.yaml` (or equivalent), so each module can carry its own linter config. Output streams in real time — nothing is buffered.

## Requirements

- [`golangci-lint`](https://golangci-lint.run/usage/install/) installed and available on `PATH`
- Go 1.26 or later

## Installation

```sh
go install github.com/StevenACoffman/lintme@latest
```

This places the `lintme` binary in `$GOPATH/bin` (or `$GOBIN`); ensure that directory is on your `PATH`.

## Commands

Running bare `lintme` is equivalent to `lintme branch` — it detects the merge-base with the remote default branch and reports only new issues introduced on the current branch.

### `lintme run` (default)

Lint all modules in the workspace. Running bare `lintme` is equivalent to `lintme run` — the subcommand is optional.

```sh
# Lint only issues introduced on the current branch (default)
lintme

# Check current branch only — do not modify files
lintme --no-fix

# Specify a base branch explicitly
lintme -B main

# Forward extra flags to every golangci-lint invocation
lintme -- --timeout=5m
lintme --no-fix -- --timeout=5m --out-format=json

# Only report issues introduced since a given commit
lintme run --new-from-rev=main
```

| Flag | Default | Description |
|------|---------|-------------|
| `--no-fix` | off | Skip `--fix`; report issues without modifying files |
| `--new-from-rev=<rev>` | — | Pass `--new-from-rev=<rev>` to every golangci-lint invocation; only issues introduced since `<rev>` are reported |

### `lintme pr <pr-number>`

Fetch the merge-base commit of a GitHub pull request and lint only the issues introduced by that PR. Equivalent to running `lintme --new-from-rev=<merge-base>` but resolves the merge-base automatically from the GitHub API.

When `<pr-number>` is omitted, `lintme pr` behaves like `gh pr view` with no arguments: it resolves the current branch via `git rev-parse --abbrev-ref HEAD`, queries the GitHub API for an open pull request whose head branch matches, and prints the detected PR number to stderr before proceeding.

```sh
# Lint the PR for the current branch (number detected automatically)
lintme pr

# Lint only issues introduced by PR #42 (repo inferred from git remote origin)
lintme pr 42

# Specify the repository explicitly
lintme pr 42 --repo=owner/repo

# Use a GitHub token for authentication
lintme pr --token=ghp_...

# Forward extra flags to golangci-lint
lintme pr 42 -- --timeout=5m
```

| Flag | Default | Description |
|------|---------|-------------|
| `--token=<token>` | `$GITHUB_TOKEN` | GitHub personal access token |
| `--repo=<owner/repo>` | detected from `git remote origin` | Repository to look up |
| `--github-url=<url>` | `$GITHUB_API_URL` | GitHub Enterprise base URL (e.g. `https://github.example.com`) |
| `--no-fix` | off | Skip `--fix` |

Without a token the GitHub API allows 60 unauthenticated requests per hour, which is enough for a single PR lookup but may be limiting in busy CI environments.

`--new-from-rev` and `pr` are mutually exclusive — `pr` sets `--new-from-rev` automatically.

Branch detection fails with an error if HEAD is detached or if no open PR is found for the current branch. Pass an explicit `<pr-number>` to skip detection in those cases.

### `lintme version`

Print build and version information.

```sh
lintme version         # human-readable table
lintme version --json  # machine-readable JSON
```

## Module and Config Discovery

### Module discovery

`lintme` walks up from the current directory looking for a `go.work` file. If found, every module listed under a `use` directive is linted sequentially in declaration order. If no `go.work` is found, it continues walking up until it finds a `go.mod` and lints that single module.

### Config discovery

For each module, `lintme` walks up from the module directory looking for a config file in this priority order:

1. `.golangci.yml`
2. `.golangci.yaml`
3. `.golangci.toml`
4. `.golangci.json`

The resolved path is shown in the output header for each module. If no config file is found, `golangci-lint` is invoked without `--config` and applies its own defaults.

## Environment Variables

Every flag can also be set via a `LINTME_`-prefixed environment variable. The mapping rule is: prepend `LINTME_`, uppercase, replace dashes with underscores.

| Flag | Environment variable |
|------|----------------------|
| `--no-fix` | `LINTME_NO_FIX=true` |
| `--new-from-rev` | `LINTME_NEW_FROM_REV=<rev>` |

For `pr`, the standard GitHub environment variables are also honoured directly:

| Flag | Environment variable |
|------|----------------------|
| `--token` | `GITHUB_TOKEN` |
| `--github-url` | `GITHUB_API_URL` |

Flags supplied on the command line always take precedence over environment variables.

## Output

```
==> ./services/auth (github.com/example/myapp/services/auth)  config: ./services/auth/.golangci.yaml
... golangci-lint output ...

==> ./services/payments (github.com/example/myapp/services/payments)  config: no config
... golangci-lint output ...

2/2 modules passed
```

When a module fails:

```
==> ./services/payments (github.com/example/myapp/services/payments)  config: no config
./services/payments/handler.go:42:9: unused variable (deadcode)
FAIL  ./services/payments: golangci-lint: exit status 1

1/2 modules passed
```

Failures in individual modules are reported but do not stop remaining modules from being linted. `lintme` exits non-zero if any module fails.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All modules passed |
| 1 | One or more modules failed |

## CI Integration

Use `lintme run --no-fix` in CI to lint all modules without modifying files. Use bare `lintme` (i.e. `lintme branch`) when you only want to report issues introduced on the current branch.

```yaml
# Lint and fix the whole workspace
- name: Lint changed files
  run: lintme

# Lint the whole workspace
- name: Lint
  run: lintme --no-fix

# Lint only issues introduced by the current PR (number passed explicitly)
- name: Lint PR
  run: lintme pr ${{ github.event.pull_request.number }} --no-fix
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

# Alternative: let lintme detect the PR from the current branch
- name: Lint PR
  run: lintme pr --no-fix
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## License

See [LICENSE](./LICENSE).
