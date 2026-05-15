# lintme

A convenience wrapper around `golangci-lint` that discovers Go modules in your workspace and runs the linter per module with zero configuration fuss.

## Overview

In a Go workspace (`go.work`) with multiple modules, running `golangci-lint` requires `cd`-ing into each module directory and invoking it separately. `lintme` automates that loop — one command from anywhere in the workspace lints everything.

Module discovery is workspace-aware: if a `go.work` file is found, `lintme` lints every module listed under a `use` directive, in order. If no `go.work` is found, it walks up from the current directory until it finds a `go.mod` and lints that single module.

For each module, `lintme` also walks up the directory tree to find the nearest `.golangci.yaml` (or equivalent), so each module can carry its own linter config. Output streams in real time — nothing is buffered.

## Requirements

- [`golangci-lint`](https://golangci-lint.run/usage/install/) installed and available on `PATH`
- Go 1.26.2 or later

## Installation

```sh
go install github.com/StevenACoffman/lintme@latest
```

This places the `lintme` binary in `$GOPATH/bin` (or `$GOBIN`); ensure that directory is on your `PATH`.

## Usage

Running bare `lintme` is equivalent to `lintme branch` — it detects the merge-base with the remote default branch and reports only new issues introduced on the current branch.

```sh
# Lint only issues introduced on the current branch (default)
lintme

# Specify a base branch explicitly
lintme -B main

# Lint all modules unconditionally, applying --fix
lintme run

# Check only — do not modify files
lintme run --no-fix

# Forward extra flags to every golangci-lint invocation
lintme -- --timeout=5m

# Print the version
lintme version
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

Output is streamed in real time as each module is linted.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All modules passed |
| 1 | One or more modules failed |

## CI Integration

Use `lintme run --no-fix` in CI to lint all modules without modifying files. Use bare `lintme` (i.e. `lintme branch`) when you only want to report issues introduced on the current branch.

```yaml
- name: Lint changed files
  run: lintme

- name: Lint all modules
  run: lintme run --no-fix
```

## License

See [LICENSE](./LICENSE).
