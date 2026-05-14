package prdiff

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/go-github/v86/github"
)

// GetPRForBranch returns the number of the open pull request whose head branch
// matches branch in the given repository. It uses the GitHub API to list open
// PRs filtered by head, matching the behavior of "gh pr view" with no number.
// Returns an error if no open PR is found for the branch.
func (c *Client) GetPRForBranch(ctx context.Context, owner, repo, branch string) (int, error) {
	opts := &github.PullRequestListOptions{
		Head:        owner + ":" + branch,
		State:       "open",
		ListOptions: github.ListOptions{PerPage: 1},
	}
	prs, _, err := c.pr.List(ctx, owner, repo, opts)
	if err != nil {
		return 0, fmt.Errorf("list pull requests for branch %q: %w", branch, err)
	}
	if len(prs) == 0 {
		return 0, fmt.Errorf(
			"no open pull request found for branch %q in %s/%s",
			branch,
			owner,
			repo,
		)
	}
	return prs[0].GetNumber(), nil
}

// GetMergeBase returns the merge-base commit SHA between the base branch and
// the head commit of the given pull request number.
func (c *Client) GetMergeBase(ctx context.Context, owner, repo string, number int) (string, error) {
	pr, _, err := c.pr.Get(ctx, owner, repo, number)
	if err != nil {
		return "", fmt.Errorf("get pull request: %w", err)
	}
	baseSHA := pr.GetBase().GetSHA()
	headSHA := pr.GetHead().GetSHA()

	cmp, _, err := c.repo.CompareCommits(ctx, owner, repo, baseSHA, headSHA, nil)
	if err != nil {
		return "", fmt.Errorf("compare commits %s...%s: %w", baseSHA, headSHA, err)
	}

	mergeBase := cmp.GetMergeBaseCommit().GetSHA()
	if mergeBase == "" {
		return "", errors.New("GitHub returned an empty merge-base commit SHA")
	}
	return mergeBase, nil
}
