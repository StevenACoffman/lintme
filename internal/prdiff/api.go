package prdiff

import (
	"context"

	"github.com/google/go-github/v86/github"
)

type reposService interface {
	CompareCommits(
		ctx context.Context,
		owner, repo, base, head string,
		opts *github.ListOptions,
	) (*github.CommitsComparison, *github.Response, error)
}

type prsService interface {
	Get(
		ctx context.Context,
		owner, repo string,
		number int,
	) (*github.PullRequest, *github.Response, error)
}
