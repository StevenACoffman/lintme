// Package run implements the "run" CLI command.
// It discovers Go modules in the current workspace (go.work or go.mod),
// finds an associated golangci-lint config file for each module, and
// executes "golangci-lint run [--fix] [--config=<path>] ./..." per module.
package run

import (
	"context"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/lintme/cmd/root"
	"github.com/StevenACoffman/lintme/internal/lintrun"
)

// Config bundles the ff/v4 flag set, command node, and the shared root config
// for the run subcommand.
type Config struct {
	*root.Config
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New self-registers the run subcommand under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("run").SetParent(parent.Flags)
	cfg.Command = &ff.Command{
		Name:      "run",
		Usage:     "lintme run [--no-fix] [--new-from-rev=<rev>] [-- <golangci-lint flags>]",
		ShortHelp: "run golangci-lint for every module in the workspace",
		LongHelp: `Run "golangci-lint run --fix ./..." for every Go module reachable
from the current working directory. Modules are linted sequentially and
output is streamed in real time.

Discovery rules:

  1. lintme walks upward from the current directory looking for go.work.
     If found, every module listed under a "use" directive is included.
  2. If no go.work is found, lintme walks upward looking for go.mod and
     uses that single module.

Config file search:

  For each module directory, lintme walks upward (toward /) checking all
  names (.golangci.yml, .golangci.yaml, .golangci.toml, .golangci.json)
  at each level before moving to the parent. The first file found is passed
  via --config=<path>. If none is found, the --config flag is omitted and
  golangci-lint uses its own default discovery.

Failures in individual modules are reported but do not stop remaining modules.
lintme exits non-zero if any module fails.

Flags after -- are forwarded verbatim to every golangci-lint invocation:

  lintme run -- --timeout=5m

golangci-lint must be present in PATH before running this command.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, extraArgs []string) error {
	return lintrun.RunModules( //nolint:wrapcheck // exec delegates entirely to RunModules; wrapping would obscure the original error
		ctx,
		cfg.Config,
		cfg.NewFromRev,
		extraArgs,
	)
}
