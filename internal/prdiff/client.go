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

// NewClient creates a new Client.
// token is a GitHub personal access token; pass an empty string for unauthenticated requests
// (60 requests/hour limit applies).
// baseURL is a base URL for GitHub Enterprise; if empty, github.com is used.
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
