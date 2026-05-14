// Package branch implements the "branch" CLI command.
// It finds the merge-base between HEAD and a base branch (defaulting to the
// repository's default branch) and runs golangci-lint with
// --new-from-rev=<merge-base> so that only issues introduced on the current
// branch are reported.
package branch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/lintme/cmd/root"
	"github.com/StevenACoffman/lintme/internal/lintrun"
)

const longHelp = `Find the merge-base between the current branch and a base branch, then run
golangci-lint with --new-from-rev=<merge-base> so that only issues introduced
on the current branch are reported.

Base branch:

  By default, the base branch is the repository's default branch, resolved by
  running:

    git symbolic-ref refs/remotes/origin/HEAD

  This returns a ref such as "refs/remotes/origin/main". If the command fails
  because the remote tracking ref is not set, run:

    git remote set-head origin --auto

  and retry. Alternatively, pass --base to skip this lookup entirely:

    lintme branch --base develop
    lintme branch -B origin/develop

  Any ref accepted by git merge-base is valid: branch names, remote tracking
  branches (origin/main), or full refs (refs/remotes/origin/main).

Merge-base:

  lintme runs:

    git merge-base HEAD <base-ref>

  to find the common ancestor. This is the same commit that "git diff main..."
  uses as its base.

Note: --new-from-rev and the branch command are mutually exclusive. The branch
command sets --new-from-rev automatically from the merge-base.

Flags after -- are forwarded verbatim to every golangci-lint invocation:

  lintme branch -- --timeout=5m

golangci-lint must be present in PATH before running this command.`

// Config holds the configuration for the branch command.
type Config struct {
	*root.Config
	base    string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New creates and registers the branch command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("branch").SetParent(parent.Flags)
	cfg.Flags.StringVar(
		&cfg.base,
		'B',
		"base",
		"",
		"base branch for the merge-base computation (default: repository's default branch via git symbolic-ref refs/remotes/origin/HEAD)",
	)
	cfg.Command = &ff.Command{
		Name:      "branch",
		Usage:     "lintme branch [-B <base>] [--no-fix] [-- <golangci-lint flags>]",
		ShortHelp: "lint only the issues introduced on the current branch",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if cfg.NewFromRev != "" {
		return errors.New(
			"branch: --new-from-rev and the branch command are mutually exclusive; the branch command sets --new-from-rev automatically from the merge-base",
		)
	}

	ref, err := cfg.resolveRef(ctx)
	if err != nil {
		return err
	}

	sha, err := mergeBase(ctx, ref)
	if err != nil {
		return err
	}

	cfg.NewFromRev = sha
	return lintrun.RunModules( //nolint:wrapcheck // exec delegates entirely to RunModules; wrapping would obscure the original error
		ctx,
		cfg.Config,
		args,
	)
}

// resolveRef returns the base ref to use for the merge-base computation.
// When --base is set it validates and returns that value directly; otherwise
// it falls back to the repository's default branch via git symbolic-ref.
func (cfg *Config) resolveRef(ctx context.Context) (string, error) {
	if cfg.base != "" {
		if err := validateRef(cfg.base); err != nil {
			return "", err
		}
		return cfg.base, nil
	}
	return defaultBranch(ctx)
}

func defaultBranch(ctx context.Context) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", fmt.Errorf(
				"branch: resolving default branch: %w: %s\n       run 'git remote set-head origin --auto' to set it, or use --base to specify a branch explicitly",
				err,
				detail,
			)
		}
		return "", fmt.Errorf(
			"branch: resolving default branch: %w\n       run 'git remote set-head origin --auto' to set it, or use --base to specify a branch explicitly",
			err,
		)
	}
	ref := strings.TrimSpace(stdout.String())
	if ref == "" {
		return "", errors.New(
			"branch: git symbolic-ref refs/remotes/origin/HEAD returned empty output",
		)
	}
	return ref, nil
}

// exec.CommandContext passes args directly to the process with no shell, so
// there is no injection risk, but gosec G204 requires variable subprocess
// arguments be validated before use.
func validateRef(ref string) error {
	if strings.ContainsAny(ref, " \t\n\r\x00;|&$`<>\\\"'()") {
		return fmt.Errorf("branch: --base %q contains characters not valid in a git ref", ref)
	}
	return nil
}

func mergeBase(ctx context.Context, ref string) (string, error) {
	var stdout, stderr bytes.Buffer
	//nolint:gosec // ref is either from git symbolic-ref output or user input validated by validateRef
	cmd := exec.CommandContext(ctx, "git", "merge-base", "HEAD", ref)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", fmt.Errorf("branch: finding merge-base with %s: %w: %s", ref, err, detail)
		}
		return "", fmt.Errorf("branch: finding merge-base with %s: %w", ref, err)
	}
	sha := strings.TrimSpace(stdout.String())
	if sha == "" {
		return "", fmt.Errorf("branch: git merge-base HEAD %s returned empty output", ref)
	}
	return sha, nil
}
