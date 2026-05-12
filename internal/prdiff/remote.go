package prdiff

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseOwnerRepo extracts the owner and repository name from a GitHub remote
// URL or a plain "owner/repo" string. It handles the following forms:
//
//	owner/repo
//	https://github.com/owner/repo
//	https://github.com/owner/repo.git
//	git@github.com:owner/repo.git
//	ssh://git@github.com/owner/repo.git
func ParseOwnerRepo(remoteURL string) (owner, repo string, err error) {
	// SCP-style: git@host:owner/repo.git
	if strings.HasPrefix(remoteURL, "git@") {
		rest := strings.TrimPrefix(remoteURL, "git@")
		colonIdx := strings.IndexByte(rest, ':')
		if colonIdx < 0 {
			return "", "", fmt.Errorf(
				"cannot parse SCP-style remote URL %q: missing ':'",
				remoteURL,
			)
		}
		return splitOwnerRepo(rest[colonIdx+1:])
	}

	// Plain owner/repo with no scheme — parse directly.
	if !strings.Contains(remoteURL, "://") {
		return splitOwnerRepo(remoteURL)
	}

	// HTTPS or SSH URL.
	u, err := url.Parse(remoteURL)
	if err != nil {
		return "", "", fmt.Errorf("cannot parse remote URL %q: %w", remoteURL, err)
	}
	return splitOwnerRepo(u.Path)
}

func splitOwnerRepo(path string) (owner, repo string, err error) {
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected owner/repo, got %q", path)
	}
	return parts[0], parts[1], nil
}
