// Package branch implements the "branch" CLI command.
// It finds the merge-base between HEAD and a base branch (defaulting to the
// repository's default branch) and runs golangci-lint with
// --new-from-rev=<merge-base> so that only issues introduced on the current
// branch are reported.
// It detects the merge-base between the current branch and the repository's
// default remote branch, then runs golangci-lint with --new-from-rev set to
// that merge-base so only new issues are reported.
package branch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
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

// refAllowed rejects ref strings that could be misinterpreted as shell
// metacharacters or git options when passed to exec.CommandContext.
var refAllowed = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)

// Config bundles the ff/v4 flag set, command node, optional base-branch
// override (-B/--base), and the shared root config for the branch subcommand.
type Config struct {
	*root.Config
	base    string
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New self-registers the branch subcommand under parent.
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
		Usage:     "lintme branch [-B <base>] [-- <golangci-lint flags>]",
		ShortHelp: "lint only issues introduced on the current branch",
		LongHelp: `Run golangci-lint with --new-from-rev set to the merge-base between
the current branch and the repository's default remote branch, so only
issues introduced on the current branch are reported.

Default branch detection:

  lintme branch queries the remote with "git ls-remote --symref origin HEAD"
  to determine the default branch without requiring a local checkout of it.
  If detection fails (e.g. no network, no remote named origin), use -B to
  supply the base branch explicitly.

Base branch override:

  lintme branch -B main
  lintme branch --base develop

Flags after -- are forwarded verbatim to every golangci-lint invocation:

  lintme branch -- --timeout=5m

golangci-lint must be present in PATH before running this command.`,
		Flags: cfg.Flags,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, extraArgs []string) error {
	if cfg.NewFromRev != "" {
		return errors.New(
			"branch: --new-from-rev and the branch command are mutually exclusive; the branch command sets --new-from-rev automatically from the merge-base",
		)
	}

	ref, err := cfg.resolveRef(ctx)
	if err != nil {
		return err
	}
	mergeBaseRev, err := mergeBase(ctx, ref)
	if err != nil {
		return err
	}
	cfg.NewFromRev = mergeBaseRev
	return lintrun.RunModules( //nolint:wrapcheck // exec delegates entirely to RunModules; wrapping would obscure the original error
		ctx,
		cfg.Config,
		extraArgs,
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

func validateRef(ref string) error {
	if !refAllowed.MatchString(ref) {
		return fmt.Errorf("branch: invalid base ref %q (allowed: letters, digits, _, ., /, -)", ref)
	}
	return nil
}

func defaultBranch(ctx context.Context) (string, error) {
	var stdout, stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--symref", "origin", "HEAD")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", fmt.Errorf(
				"branch: querying remote default branch: %w: %s\n       use --base to specify a branch explicitly",
				err,
				detail,
			)
		}
		return "", fmt.Errorf(
			"branch: querying remote default branch: %w\n       use --base to specify a branch explicitly",
			err,
		)
	}
	// Output format: "ref: refs/heads/main\tHEAD"
	for _, line := range strings.Split(stdout.String(), "\n") {
		headsRef, ok := strings.CutPrefix(line, "ref: ")
		if !ok {
			continue
		}
		headsRef, _, ok = strings.Cut(headsRef, "\t")
		if !ok {
			continue
		}
		branch, ok := strings.CutPrefix(headsRef, "refs/heads/")
		if !ok {
			continue
		}
		return "origin/" + branch, nil
	}
	return "", errors.New(
		"branch: git ls-remote --symref origin HEAD did not return a symbolic ref\n       use --base to specify a branch explicitly",
	)
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
