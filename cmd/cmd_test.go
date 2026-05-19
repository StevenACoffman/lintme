package cmd

import (
	"testing"

	"github.com/peterbourgon/ff/v4"
)

func TestDefaultSubcommand(t *testing.T) {
	subs := []*ff.Command{
		{Name: "run"},
		{Name: "branch"},
		{Name: "pr"},
		{Name: "version"},
	}

	cases := map[string]struct {
		args []string
		want []string
	}{
		"known subcommand first — unchanged": {
			args: []string{"run", "--no-fix"},
			want: []string{"run", "--no-fix"},
		},
		"flags then known subcommand — unchanged": {
			args: []string{"--no-fix", "run"},
			want: []string{"--no-fix", "run"},
		},
		"no args — fallback prepended": {
			args: []string{},
			want: []string{"branch"},
		},
		"flags only — fallback prepended": {
			args: []string{"--no-fix", "--fmt-only"},
			want: []string{"branch", "--no-fix", "--fmt-only"},
		},
		"unknown first arg — fallback prepended": {
			args: []string{"unknown", "--no-fix"},
			want: []string{"branch", "unknown", "--no-fix"},
		},
		"passthrough separator stops detection — fallback prepended": {
			// 'run' after '--' is not treated as a subcommand name
			args: []string{"--", "run"},
			want: []string{"branch", "--", "run"},
		},
		"known subcommand after passthrough — unchanged (subcommand before --)": {
			args: []string{"run", "--", "--timeout=5m"},
			want: []string{"run", "--", "--timeout=5m"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := defaultSubcommand(subs, tc.args, "branch")
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
