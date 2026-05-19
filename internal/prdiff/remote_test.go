package prdiff

import (
	"testing"
)

func TestParseOwnerRepo(t *testing.T) {
	cases := map[string]struct {
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		"plain owner/repo": {
			input: "owner/repo", wantOwner: "owner", wantRepo: "repo",
		},
		"https URL": {
			input: "https://github.com/owner/repo", wantOwner: "owner", wantRepo: "repo",
		},
		"https URL with .git suffix": {
			input: "https://github.com/owner/repo.git", wantOwner: "owner", wantRepo: "repo",
		},
		"SCP-style git remote": {
			input: "git@github.com:owner/repo.git", wantOwner: "owner", wantRepo: "repo",
		},
		"SSH URL": {
			input: "ssh://git@github.com/owner/repo.git", wantOwner: "owner", wantRepo: "repo",
		},
		"GitHub Enterprise https URL": {
			input: "https://github.example.com/org/project", wantOwner: "org", wantRepo: "project",
		},
		"missing slash — error": {
			input: "justarepo", wantErr: true,
		},
		"SCP-style missing colon — error": {
			input: "git@github.com/owner/repo", wantErr: true,
		},
		"empty string — error": {
			input: "", wantErr: true,
		},
		"owner only with trailing slash — error": {
			input: "owner/", wantErr: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			owner, repo, err := ParseOwnerRepo(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf(
						"expected error for input %q, got owner=%q repo=%q",
						tc.input,
						owner,
						repo,
					)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if owner != tc.wantOwner {
				t.Errorf("owner: got %q, want %q", owner, tc.wantOwner)
			}
			if repo != tc.wantRepo {
				t.Errorf("repo: got %q, want %q", repo, tc.wantRepo)
			}
		})
	}
}

func TestSplitOwnerRepo(t *testing.T) {
	cases := map[string]struct {
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		"plain": {
			input: "owner/repo", wantOwner: "owner", wantRepo: "repo",
		},
		"leading slash stripped": {
			input: "/owner/repo", wantOwner: "owner", wantRepo: "repo",
		},
		".git suffix stripped": {
			input: "owner/repo.git", wantOwner: "owner", wantRepo: "repo",
		},
		"both leading slash and .git": {
			input: "/owner/repo.git", wantOwner: "owner", wantRepo: "repo",
		},
		"no slash — error": {
			input: "justarepo", wantErr: true,
		},
		"empty owner — error": {
			input: "/repo", wantErr: true,
		},
		"empty repo — error": {
			input: "owner/", wantErr: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			owner, repo, err := splitOwnerRepo(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf(
						"expected error for input %q, got owner=%q repo=%q",
						tc.input,
						owner,
						repo,
					)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if owner != tc.wantOwner {
				t.Errorf("owner: got %q, want %q", owner, tc.wantOwner)
			}
			if repo != tc.wantRepo {
				t.Errorf("repo: got %q, want %q", repo, tc.wantRepo)
			}
		})
	}
}
