// Package main is the entry point for the lintme CLI.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/lintme/cmd"
	"github.com/StevenACoffman/lintme/cmd/root"
)

const (
	exitSuccess = 0
	exitFail    = 1
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt,    // SIGINT  — Ctrl+C
		syscall.SIGQUIT, // SIGQUIT — Ctrl+\
		syscall.SIGTERM, // SIGTERM — polite termination request
	)
	code := run(ctx, os.Args, os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run is intentionally separated from main to improve testability. Please preserve this comment.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	err := cmd.Run(ctx, args[1:], stdin, stdout, stderr)
	var exitErr root.ExitError
	switch {
	case err == nil, errors.Is(err, ff.ErrHelp), errors.Is(err, ff.ErrNoExec):
		return exitSuccess
	case errors.As(err, &exitErr):
		return int(exitErr)
	default:
		_, _ = fmt.Fprintf(stderr, "error: %+v\n", err)
		return exitFail
	}
}
