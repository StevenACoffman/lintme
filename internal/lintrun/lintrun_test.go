package lintrun

import (
	"path/filepath"
	"testing"
)

func TestLintCandidates(t *testing.T) {
	home := "/home/user"
	khan := filepath.Join(home, "khan", "webapp", "genfiles", "go", "bin", "golangci-lint")
	gobin := filepath.Join(home, "go", "bin", "golangci-lint")
	brew := "/opt/homebrew/bin/golangci-lint"
	pathHit := "/usr/local/bin/golangci-lint" // a PATH result that is not the Khan binary

	cases := map[string]struct {
		home           string
		khanPath       string
		lookPathResult string
		inKhan         bool
		want           []string
	}{
		"outside khan — unique PATH result": {
			home: home, khanPath: khan, lookPathResult: pathHit, inKhan: false,
			want: []string{pathHit, gobin, brew, khan},
		},
		"outside khan — PATH result is khan path, demoted to last": {
			home: home, khanPath: khan, lookPathResult: khan, inKhan: false,
			want: []string{gobin, brew, khan},
		},
		"outside khan — PATH not found": {
			home: home, khanPath: khan, lookPathResult: "", inKhan: false,
			want: []string{gobin, brew, khan},
		},
		"inside khan — khan path leads": {
			home: home, khanPath: khan, lookPathResult: pathHit, inKhan: true,
			want: []string{khan, pathHit, gobin, brew},
		},
		"inside khan — no PATH result": {
			home: home, khanPath: khan, lookPathResult: "", inKhan: true,
			want: []string{khan, gobin, brew},
		},
		"inside khan — PATH result is khan path, both included": {
			// inside the workspace khanPath always leads; the PATH result is
			// also included because inKhan=true satisfies the second condition,
			// producing a duplicate entry that is harmless (first stat wins).
			home: home, khanPath: khan, lookPathResult: khan, inKhan: true,
			want: []string{khan, khan, gobin, brew},
		},
		"no home — only PATH and homebrew": {
			home: "", khanPath: "", lookPathResult: pathHit, inKhan: false,
			want: []string{pathHit, brew},
		},
		"no home, inside khan — only homebrew": {
			home: "", khanPath: "", lookPathResult: "", inKhan: true,
			want: []string{brew},
		},
		"no home, inside khan — PATH result included": {
			home: "", khanPath: "", lookPathResult: pathHit, inKhan: true,
			want: []string{pathHit, brew},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := lintCandidates(tc.home, tc.khanPath, tc.lookPathResult, tc.inKhan)
			if len(got) != len(tc.want) {
				t.Fatalf("len(got)=%d, len(want)=%d\n got:  %v\n want: %v",
					len(got), len(tc.want), got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}
