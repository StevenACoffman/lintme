package prdiff

import (
	"context"
	"errors"
	"fmt"
)

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
