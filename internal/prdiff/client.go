package prdiff

import (
	"fmt"

	"github.com/google/go-github/v86/github"
)

// Client retrieves the merge-base commit SHA of a GitHub pull request.
type Client struct {
	pr   prsService
	repo reposService
}

// NewClient initializes a GitHub API client.
// Pass an empty token for unauthenticated access (60 req/hour limit applies).
// Pass a non-empty baseURL to target a GitHub Enterprise instance instead of github.com.
func NewClient(token, baseURL string) (*Client, error) {
	gh := github.NewClient(nil)
	if token != "" {
		gh = gh.WithAuthToken(token)
	}
	if baseURL != "" {
		g, err := gh.WithEnterpriseURLs(baseURL, "")
		if err != nil {
			return nil, fmt.Errorf("set enterprise URL: %w", err)
		}
		gh = g
	}
	return &Client{
		pr:   gh.PullRequests,
		repo: gh.Repositories,
	}, nil
}
