// Package root defines the root configuration for the CLI.
package root

import (
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"
)

// ExitError is returned by commands that want a specific non-zero exit code
// without printing an additional error message. run() in main.go checks for
// ExitError with errors.As and calls os.Exit(int(e)) directly, bypassing the
// default "error: ..." printer.
type ExitError int

// Config holds shared I/O writers, shared flags, and the root ff.Command.
// All subcommand configs embed *Config to inherit these.
type Config struct {
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	NoFix      bool
	NewFromRev string
	Flags      *ff.FlagSet
	Command    *ff.Command
}

func (e ExitError) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

// New wires I/O into the root config and registers the shared --no-fix and
// --new-from-rev flags inherited by all subcommands.
func New(stdin io.Reader, stdout, stderr io.Writer) *Config {
	var cfg Config
	cfg.Stdin = stdin
	cfg.Stdout = stdout
	cfg.Stderr = stderr
	cfg.Flags = ff.NewFlagSet("lintme")
	cfg.Flags.BoolVar(&cfg.NoFix, 0, "no-fix", "skip --fix; check only, do not modify files")
	cfg.Flags.StringVar(
		&cfg.NewFromRev,
		0,
		"new-from-rev",
		"",
		"show only new issues introduced since `rev` (passed to golangci-lint --new-from-rev)",
	)
	cfg.Command = &ff.Command{
		Name:      "lintme",
		Usage:     "lintme [--no-fix] [--new-from-rev=<rev>] [-- <golangci-lint flags>]",
		ShortHelp: "run golangci-lint across every module in a Go workspace",
		Flags:     cfg.Flags,
	}
	return &cfg
}
