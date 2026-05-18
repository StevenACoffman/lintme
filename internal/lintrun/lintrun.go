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
	"time"

	"golang.org/x/mod/modfile"

	"github.com/StevenACoffman/lintme/cmd/root"
)

// ModuleEntry holds the module path and the filesystem directory for one Go module.
type ModuleEntry struct {
	ModulePath string // e.g. "github.com/example/myapp"
	Dir        string // absolute path to the directory containing go.mod
}

// RunModules discovers Go modules reachable from the current directory,
// then runs golangci-lint for each one sequentially, streaming output in
// real time. It reads cfg.NoFix, cfg.FmtOnly, and cfg.NewFromRev from the
// shared root config.
func RunModules(ctx context.Context, cfg *root.Config, extraArgs []string) error {
	if cfg.FmtOnly && cfg.NoFix {
		return errors.New("--fmt-only and --no-fix are mutually exclusive")
	}

	lintPath, err := exec.LookPath("golangci-lint")
	if err != nil {
		return fmt.Errorf("golangci-lint not found in PATH: %w", err)
	}

	//nolint:forbidigo // not a test fixture path; os.Getwd is needed to locate the workspace root
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
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
			cfg.NewFromRev,
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

func printHeader(w io.Writer, mod ModuleEntry, configPath string) {
	configDesc := "no config"
	if configPath != "" {
		configDesc = configPath
	}
	_, _ = fmt.Fprintf(w, "==> %s (%s)  config: %s\n", mod.Dir, mod.ModulePath, configDesc)
}

func discoverModules(dir string) ([]ModuleEntry, error) {
	if workPath, found := walkUpFind(dir, "go.work"); found {
		return discoverFromWorkFile(workPath)
	}
	if modPath, found := walkUpFind(dir, "go.mod"); found {
		modulePath, err := readModulePath(modPath)
		if err != nil {
			return nil, err
		}
		return []ModuleEntry{{ModulePath: modulePath, Dir: filepath.Dir(modPath)}}, nil
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

func discoverFromWorkFile(workPath string) ([]ModuleEntry, error) {
	data, err := os.ReadFile(workPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", workPath, err)
	}
	wf, err := modfile.ParseWork(workPath, data, nil)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", workPath, err)
	}
	workDir := filepath.Dir(workPath)
	var entries []ModuleEntry
	for _, use := range wf.Use {
		// use.Path is relative to the go.work directory.
		moduleDir := filepath.Join(workDir, filepath.FromSlash(use.Path))
		goModPath := filepath.Join(moduleDir, "go.mod")
		modulePath, err := readModulePath(goModPath)
		if err != nil {
			return nil, err
		}
		entries = append(entries, ModuleEntry{ModulePath: modulePath, Dir: moduleDir})
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

	cmd := exec.CommandContext(
		ctx,
		lintPath,
		args...) //nolint:gosec // lintPath comes from exec.LookPath; args are constructed internally
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
