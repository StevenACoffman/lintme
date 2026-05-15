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

PR number:

  When <pr-number> is omitted, lintme detects the open pull request that
  belongs to the current git branch, matching the behavior of "gh pr view"
  with no arguments. The current branch is resolved via "git rev-parse
  --abbrev-ref HEAD" and the GitHub API is queried for an open PR whose head
  branch matches. An error is returned if no open PR is found for the branch
  or if HEAD is detached.

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

// Config bundles the ff/v4 flag set, command node, GitHub credentials, and
// the shared root config for the pr subcommand.
type Config struct {
	*root.Config
	token     string
	repo      string
	githubURL string
	Flags     *ff.FlagSet
	Command   *ff.Command
}

// New self-registers the pr subcommand under parent.
func New(parent *root.Config) *Config {
	var cfg Config
	cfg.Config = parent
	cfg.Flags = ff.NewFlagSet("pr").SetParent(parent.Flags)
	cfg.Flags.StringVar(
		&cfg.token,
		0,
		"token",
		"",
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
		"",
		"GitHub API base URL for GitHub Enterprise (default: $GITHUB_API_URL)",
	)
	cfg.Command = &ff.Command{
		Name:      "pr",
		Usage:     "lintme pr [--token=<token>] [--repo=owner/repo] [--no-fix] [<pr-number>] [-- <golangci-lint flags>]",
		ShortHelp: "lint only the files changed by a pull request",
		LongHelp:  longHelp,
		Flags:     cfg.Flags,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return &cfg
}

func (cfg *Config) exec(ctx context.Context, args []string) error {
	var prNum int
	var extraArgs []string
	if len(args) > 0 {
		if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
			prNum = n
			extraArgs = args[1:]
		} else {
			extraArgs = args
		}
	}

	if cfg.NewFromRev != "" {
		return errors.New(
			"pr: --new-from-rev and the pr command are mutually exclusive; the pr command sets --new-from-rev automatically from the pull request merge-base",
		)
	}

	// GITHUB_TOKEN and GITHUB_API_URL are conventional third-party env vars;
	// checked here as fallbacks because they don't match the LINTME_ prefix.
	token := cfg.token
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	githubURL := cfg.githubURL
	if githubURL == "" {
		githubURL = os.Getenv("GITHUB_API_URL")
	}

	owner, repo, err := resolveOwnerRepo(ctx, cfg.repo)
	if err != nil {
		return err
	}

	client, err := prdiff.NewClient(token, githubURL)
	if err != nil {
		return fmt.Errorf("pr: create GitHub client: %w", err)
	}

	prNum, err = cfg.resolvePRNum(ctx, client, owner, repo, prNum)
	if err != nil {
		return err
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

func (cfg *Config) resolvePRNum(
	ctx context.Context,
	client *prdiff.Client,
	owner, repo string,
	prNum int,
) (int, error) {
	if prNum != 0 {
		return prNum, nil
	}
	branch, err := currentBranch(ctx)
	if err != nil {
		return 0, fmt.Errorf(
			"pr: %w\n       pass an explicit <pr-number> to skip branch detection",
			err,
		)
	}
	n, err := client.GetPRForBranch(ctx, owner, repo, branch)
	if err != nil {
		return 0, fmt.Errorf("pr: %s/%s: %w", owner, repo, err)
	}
	_, _ = fmt.Fprintf(cfg.Stderr, "pr: found PR #%d for branch %q\n", n, branch)
	return n, nil
}

func currentBranch(ctx context.Context) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", fmt.Errorf("detecting current branch: %w: %s", err, detail)
		}
		return "", fmt.Errorf("detecting current branch: %w", err)
	}
	branch := strings.TrimSpace(stdout.String())
	if branch == "" || branch == "HEAD" {
		return "", errors.New("not on a branch (detached HEAD state)")
	}
	return branch, nil
}

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
