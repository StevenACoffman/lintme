// Package cmd is the dispatcher for the lintme CLI.
// It registers all commands and routes incoming arguments
// to the matching command implementation.
package cmd

// climax:name lintme
// climax:root-pkg root
// climax:env-prefix LINTME

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/StevenACoffman/lintme/cmd/branch"
	"github.com/StevenACoffman/lintme/cmd/root"
	"github.com/StevenACoffman/lintme/cmd/run"
	"github.com/StevenACoffman/lintme/cmd/version"
)

// Run parses args and dispatches to the matching command.
// args must not include the executable name (pass os.Args[1:]).
//
// Every flag can be set via a LINTME_-prefixed environment variable.
// The mapping rule is: prepend LINTME_, uppercase, replace dashes with
// underscores.
//
// Flags supplied on the command line always take precedence over env vars.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	r := root.New(stdin, stdout, stderr)
	version.New(r)
	run.New(r)
	branch.New(r)
	// register new commands here

	// Default to "run" when no subcommand is explicitly given.
	// Find the first non-flag argument; if it isn't a registered subcommand
	// name, prepend "run" so that flags like --no-fix work without typing
	// the subcommand name.
	args = defaultSubcommand(r.Command.Subcommands, args, "branch")

	if err := r.Command.Parse(args, ff.WithEnvVarPrefix("LINTME")); err != nil {
		_, _ = fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(r.Command))
		return fmt.Errorf("parse: %w", err)
	}

	if err := r.Command.Run(ctx); err != nil {
		// Don't print usage help for ErrNoExec (defensive; defaultSubcommand
		// should prevent this) or ExitError (command already reported its own
		// outcome).
		var exitErr root.ExitError
		if !errors.Is(err, ff.ErrNoExec) && !errors.As(err, &exitErr) {
			_, _ = fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(r.Command.GetSelected()))
		}
		return err
	}

	return nil
}

// defaultSubcommand prepends fallback to args when the first non-flag
// argument is not the name of a registered subcommand. This makes fallback
// the implicit subcommand when the user omits an explicit one.
func defaultSubcommand(subcommands []*ff.Command, args []string, fallback string) []string {
	known := make(map[string]bool, len(subcommands))
	for _, sub := range subcommands {
		known[sub.Name] = true
	}
	for _, a := range args {
		if a == "--" {
			break // passthrough separator — nothing after this is a subcommand name
		}
		if a == "" || a[0] == '-' {
			continue // skip flags
		}
		if known[a] {
			return args // explicit subcommand found — leave args unchanged
		}
		break // first non-flag arg is not a subcommand name — use default
	}
	return append([]string{fallback}, args...)
}
