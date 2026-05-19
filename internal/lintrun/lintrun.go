// Package lintrun provides the core module-discovery and golangci-lint
// execution loop shared by the run and branch commands.
package lintrun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/mod/modfile"

	"github.com/StevenACoffman/lintme/cmd/root"
)

// moduleEntry holds the module path and the filesystem directory for one Go module.
type moduleEntry struct {
	ModulePath string // e.g. "github.com/example/myapp"
	Dir        string // absolute path to the directory containing go.mod
}

// RunModules discovers Go modules reachable from the current directory,
// then runs golangci-lint for each one sequentially, streaming output in
// real time. It reads cfg.NoFix and cfg.FmtOnly from the shared root config;
// newFromRev is passed explicitly so callers (branch, pr) can supply a
// computed merge-base without mutating the shared config.
func RunModules(
	ctx context.Context,
	cfg *root.Config,
	newFromRev string,
	extraArgs []string,
) error {
	if cfg.FmtOnly && cfg.NoFix {
		return errors.New("--fmt-only and --no-fix are mutually exclusive")
	}

	//nolint:forbidigo // not a test fixture path; os.Getwd is needed to locate the workspace root
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	home := os.Getenv("HOME") // read at boundary; passed into findLintExecutable for testability
	lintPath, err := findLintExecutable(cwd, home)
	if err != nil {
		return err
	}

	modules, err := discoverModules(cwd)
	if err != nil {
		return err
	}
	if len(modules) == 0 {
		return fmt.Errorf("no Go modules found from %s", cwd)
	}

	passed, failed := 0, 0
	for _, mod := range modules {
		configPath := findGolangciConfig(mod.Dir)
		printHeader(cfg.Stdout, mod, configPath)
		if err := runLint(
			ctx,
			lintPath,
			mod.Dir,
			configPath,
			!cfg.NoFix,
			cfg.FmtOnly,
			newFromRev,
			extraArgs,
			cfg.Stdout,
			cfg.Stderr,
		); err != nil {
			_, _ = fmt.Fprintf(cfg.Stderr, "FAIL  %s: %v\n", mod.Dir, err)
			failed++
		} else {
			passed++
		}
	}
	_, _ = fmt.Fprintf(cfg.Stdout, "\n%d/%d modules passed\n", passed, len(modules))
	if failed > 0 {
		return root.ExitError(1)
	}
	return nil
}

func printHeader(w io.Writer, mod moduleEntry, configPath string) {
	configDesc := "no config"
	if configPath != "" {
		configDesc = configPath
	}
	_, _ = fmt.Fprintf(w, "==> %s (%s)  config: %s\n", mod.Dir, mod.ModulePath, configDesc)
}

func discoverModules(dir string) ([]moduleEntry, error) {
	if workPath, found := walkUpFind(dir, "go.work"); found {
		return discoverFromWorkFile(workPath)
	}
	if modPath, found := walkUpFind(dir, "go.mod"); found {
		modulePath, err := readModulePath(modPath)
		if err != nil {
			return nil, err
		}
		return []moduleEntry{{ModulePath: modulePath, Dir: filepath.Dir(modPath)}}, nil
	}
	return nil, fmt.Errorf("no go.work or go.mod found in %s or any parent directory", dir)
}

func walkUpFind(dir, filename string) (string, bool) {
	current := dir
	for {
		candidate := filepath.Join(current, filename)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

func discoverFromWorkFile(workPath string) ([]moduleEntry, error) {
	data, err := os.ReadFile(workPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", workPath, err)
	}
	wf, err := modfile.ParseWork(workPath, data, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", workPath, err)
	}
	workDir := filepath.Dir(workPath)
	var entries []moduleEntry
	for _, use := range wf.Use {
		// use.Path is relative to the go.work directory.
		moduleDir := filepath.Join(workDir, filepath.FromSlash(use.Path))
		goModPath := filepath.Join(moduleDir, "go.mod")
		modulePath, err := readModulePath(goModPath)
		if err != nil {
			return nil, err
		}
		entries = append(entries, moduleEntry{ModulePath: modulePath, Dir: moduleDir})
	}
	return entries, nil
}

func readModulePath(goModPath string) (string, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", goModPath, err)
	}
	mf, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", goModPath, err)
	}
	if mf.Module == nil {
		return "", fmt.Errorf("%s: missing module directive", goModPath)
	}
	return mf.Module.Mod.Path, nil
}

func findGolangciConfig(dir string) string {
	configNames := []string{
		".golangci.yml",
		".golangci.yaml",
		".golangci.toml",
		".golangci.json",
	}
	current := dir
	for {
		for _, name := range configNames {
			candidate := filepath.Join(current, name)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

// findLintExecutable locates the golangci-lint binary.
//
// Search order depends on whether cwd contains "khan/webapp":
//
//   - Inside a Khan workspace: $HOME/khan/webapp/genfiles/go/bin is checked
//     first (if the file exists there), then PATH, then $HOME/go/bin, then
//     /opt/homebrew/bin.
//   - Outside a Khan workspace: PATH is checked first, but if LookPath returns
//     the Khan path it is skipped; the remaining order is $HOME/go/bin,
//     /opt/homebrew/bin, and $HOME/khan/webapp/genfiles/go/bin last.
//
// home is the value of $HOME, passed in by the caller so this function remains
// testable without manipulating environment variables.
func findLintExecutable(cwd, home string) (string, error) {
	const bin = "golangci-lint"

	inKhan := strings.Contains(cwd, "khan/webapp")
	var khanPath string
	if home != "" {
		khanPath = filepath.Join(home, "khan", "webapp", "genfiles", "go", "bin", bin)
	}
	lookPathResult, _ := exec.LookPath(bin)

	for _, candidate := range lintCandidates(home, khanPath, lookPathResult, inKhan) {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf(
		"%s not found in PATH or well-known directories"+
			" ($HOME/go/bin, $HOME/khan/webapp/genfiles/go/bin, /opt/homebrew/bin)",
		bin,
	)
}

// lintCandidates returns the ordered list of absolute paths to try for
// golangci-lint. lookPathResult is the pre-resolved output of exec.LookPath
// (empty string if the binary was not found on PATH).
//
// Inside a Khan workspace khanPath leads; outside it trails. When outside
// and lookPathResult happens to equal khanPath, the PATH result is omitted
// from its normal position so khanPath appears only as the last fallback.
func lintCandidates(home, khanPath, lookPathResult string, inKhan bool) []string {
	const bin = "golangci-lint"
	var out []string
	if inKhan && khanPath != "" {
		out = append(out, khanPath)
	}
	if lookPathResult != "" && (inKhan || lookPathResult != khanPath) {
		out = append(out, lookPathResult)
	}
	if home != "" {
		out = append(out, filepath.Join(home, "go", "bin", bin))
	}
	out = append(out, "/opt/homebrew/bin/"+bin)
	if !inKhan && khanPath != "" {
		out = append(out, khanPath)
	}
	return out
}

// runLint uses cmd.Dir rather than os.Chdir to avoid mutating global process state.
func runLint(
	ctx context.Context,
	lintPath, dir, configPath string,
	fix bool,
	fmtOnly bool,
	newFromRev string,
	extraArgs []string,
	stdout, stderr io.Writer,
) error {
	var args []string
	if fmtOnly {
		args = []string{"fmt"}
	} else {
		args = []string{"run"}
		if fix && !slices.Contains(extraArgs, "--fix") {
			args = append(args, "--fix")
		}
		if newFromRev != "" {
			args = append(args, "--new-from-rev="+newFromRev)
		}
	}
	if configPath != "" {
		args = append(args, "--config="+configPath)
	}
	args = append(args, "./...")
	args = append(args, extraArgs...)

	//nolint:gosec // lintPath is resolved by findLintExecutable (exec.LookPath or os.Stat); args are constructed internally
	cmd := exec.CommandContext(ctx, lintPath, args...)
	// Send SIGINT on cancellation so golangci-lint can flush its output before exit.
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 30 * time.Second // grace period after SIGINT before forced kill
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("golangci-lint: %w", err)
	}
	return nil
}
