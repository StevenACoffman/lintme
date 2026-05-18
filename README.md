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

Running bare `lintme` runs `lintme run` — no subcommand needed for the common case.

### `lintme run` (Default)

Lint all modules in the workspace.

```sh
# Lint all modules, applying --fix (default)
lintme

# Check only — do not modify files
lintme --no-fix

# Report only issues introduced since a given commit
lintme --new-from-rev=main

# Forward extra flags to every golangci-lint invocation
lintme -- --timeout=5m
lintme --no-fix -- --timeout=5m --out-format=json
```

| Flag                   | Default | Description                                                                                                               |
| ---------------------- | ------- | ------------------------------------------------------------------------------------------------------------------------- |
| `--no-fix`             | off     | Skip `--fix`; report issues without modifying files                                                                       |
| `--new-from-rev=<rev>` | —       | Pass `--new-from-rev=<rev>` to every golangci-lint invocation; golangci-lint reports only issues introduced since `<rev>` |

### `lintme version`

Print build and version information.

```sh
lintme version         # human-readable table
lintme version --json  # machine-readable JSON
```

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
