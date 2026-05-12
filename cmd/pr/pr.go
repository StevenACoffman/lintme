// Package pr implements the "pr" CLI command.
// It fetches the merge-base commit of a GitHub pull request via the API and
// runs golangci-lint with --new-from-rev=<merge-base> so that only issues
// introduced by the PR are reported.
package pr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v4"

	"github.com/StevenACoffman/lintme/cmd/root"
	"github.com/StevenACoffman/lintme/internal/lintrun"
	"github.com/StevenACoffman/lintme/internal/prdiff"
)

const longHelp = `Fetch the merge-base commit of a GitHub pull request via the API and run
golangci-lint with --new-from-rev=<merge-base> so that only issues introduced
by the PR are reported.

Authentication:

  Provide a GitHub personal access token via --token or the GITHUB_TOKEN
  environment variable. Without a token, the GitHub API allows 60 unauthenticated
  requests per hour, which is sufficient for a single PR lookup but may hit
  limits in busy CI environments.

Repository detection:

  If --repo is not provided, lintme infers owner/repo from the remote URL of
  the "origin" git remote in the current directory.

GitHub Enterprise:

  Set --github-url or GITHUB_API_URL to point at a GitHub Enterprise instance
  (e.g. https://github.example.com).

Note: --new-from-rev and the pr command are mutually exclusive. The pr command
sets --new-from-rev automatically from the pull request merge-base.

Flags after -- are forwarded verbatim to every golangci-lint invocation:

  lintme pr 123 -- --timeout=5m

golangci-lint must be present in PATH before running this command.`

// Config holds the configuration for the pr command.
type Config struct {
	*root.Config
	token     string
	repo      string
	githubURL string
	Flags     *ff.FlagSet
	Command   *ff.Command
}

// New creates and registers the pr command with the given parent config.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("pr").SetParent(parent.Flags)
	cfg.Flags.StringVar(
		&cfg.token,
		0,
		"token",
		os.Getenv("GITHUB_TOKEN"),
		"GitHub personal access token (default: $GITHUB_TOKEN)",
	)
	cfg.Flags.StringVar(
		&cfg.repo,
		0,
		"repo",
		"",
		"repository as owner/repo (default: detected from git remote origin)",
	)
	cfg.Flags.StringVar(
		&cfg.githubURL,
		0,
		"github-url",
		os.Getenv("GITHUB_API_URL"),
		"GitHub API base URL for GitHub Enterprise (default: $GITHUB_API_URL)",
	)
	cfg.Command = &ff.Command{
		Name:      "pr",
		Usage:     "lintme pr [--token=<token>] [--repo=owner/repo] [--no-fix] <pr-number> [-- <golangci-lint flags>]",
		ShortHelp: "lint only the files changed by a pull request",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("pr: missing required argument <pr-number>")
	}
	prNum, err := strconv.Atoi(args[0])
	if err != nil || prNum <= 0 {
		return fmt.Errorf("pr: expected a positive integer PR number, got %q", args[0])
	}
	extraArgs := args[1:]

	if cfg.NewFromRev != "" {
		return errors.New(
			"pr: --new-from-rev and the pr command are mutually exclusive; the pr command sets --new-from-rev automatically from the pull request merge-base",
		)
	}

	owner, repo, err := resolveOwnerRepo(ctx, cfg.repo)
	if err != nil {
		return err
	}

	hc := prdiff.NewHTTPClient(cfg.token)
	client, err := prdiff.NewClient(hc, cfg.githubURL)
	if err != nil {
		return fmt.Errorf("pr: create GitHub client: %w", err)
	}
	mergeBase, err := client.GetMergeBase(ctx, owner, repo, prNum)
	if err != nil {
		return fmt.Errorf("pr: %s/%s#%d: %w", owner, repo, prNum, err)
	}

	cfg.NewFromRev = mergeBase
	return lintrun.RunModules( //nolint:wrapcheck // exec delegates entirely to RunModules; wrapping would obscure the original error
		ctx,
		cfg.Config,
		extraArgs,
	)
}

// resolveOwnerRepo returns the owner and repo name from the given slug,
// or detects it from the "origin" git remote if slug is empty.
func resolveOwnerRepo(ctx context.Context, slug string) (owner, repo string, err error) {
	if slug != "" {
		owner, repo, err = prdiff.ParseOwnerRepo(slug)
		if err != nil {
			return "", "", fmt.Errorf("pr: --repo %q: %w", slug, err)
		}
		return owner, repo, nil
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", "", fmt.Errorf(
				"pr: detecting repository from git remote origin: %w: %s\n       use --repo=owner/repo to set it explicitly",
				err,
				detail,
			)
		}
		return "", "", fmt.Errorf(
			"pr: detecting repository from git remote origin: %w\n       use --repo=owner/repo to set it explicitly",
			err,
		)
	}
	remoteURL := strings.TrimSpace(stdout.String())
	owner, repo, err = prdiff.ParseOwnerRepo(remoteURL)
	if err != nil {
		return "", "", fmt.Errorf(
			"pr: parsing git remote URL %q: %w\n       use --repo=owner/repo to set it explicitly",
			remoteURL,
			err,
		)
	}
	return owner, repo, nil
}
