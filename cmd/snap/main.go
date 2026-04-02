// Snap CLI — state-aware checkpoints for AI-agent workflows.
//
// Usage:
//
//	snap init                  Initialize .snap/ directory and Git hooks
//	snap save [--force]        Capture current state for HEAD
//	snap restore <git-hash>    Restore state for a commit
//	snap verify <git-hash>     Verify blob integrity for a commit
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NishthaNabya/snap/hooks"
	"github.com/NishthaNabya/snap/orchestrator"
	"github.com/NishthaNabya/snap/snap"

	// Register drivers via init().
	_ "github.com/NishthaNabya/snap/drivers/dotenv"
	_ "github.com/NishthaNabya/snap/drivers/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	ctx := context.Background()
	cmd := os.Args[1]

	switch cmd {
	case "init":
		cmdInit(ctx)
	case "save":
		cmdSave(ctx)
	case "restore":
		cmdRestore(ctx)
	case "verify":
		cmdVerify(ctx)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "snap: unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: snap <command> [args]

Commands:
  init                 Initialize .snap/ directory and install Git hooks
  save [--force]       Capture current state and bind to HEAD
  save --auto          Capture if state changed (for Git hooks)
  restore <git-hash>   Restore state snapshot for commit
  verify <git-hash>    Verify blob integrity for commit

Snap provides state-aware checkpoints for AI-agent workflows.
It captures databases, env files, and configs alongside Git commits.`)
}

func cmdInit(ctx context.Context) {
	root, err := findRepoRoot()
	if err != nil {
		fatal(err)
	}

	orch := orchestrator.New(root, snap.Registry)
	if err := orch.Init(); err != nil {
		fatal(err)
	}

	// Install Git hooks.
	gitHooksDir := filepath.Join(root, ".git", "hooks")
	if err := os.MkdirAll(gitHooksDir, 0o755); err != nil {
		fatal(fmt.Errorf("snap: create hooks dir: %w", err))
	}
	if err := hooks.Install(gitHooksDir); err != nil {
		fatal(err)
	}

	fmt.Fprintln(os.Stderr, "snap: initialized .snap/ directory and Git hooks")
	fmt.Fprintln(os.Stderr, "snap: configure drivers in .snap/config.json")
}

func cmdSave(ctx context.Context) {
	root, err := findRepoRoot()
	if err != nil {
		fatal(err)
	}

	force := false
	auto := false
	for _, arg := range os.Args[2:] {
		switch arg {
		case "--force", "-f":
			force = true
		case "--auto":
			auto = true
		}
	}

	orch := orchestrator.New(root, snap.Registry)
	if err := orch.Save(ctx, force); err != nil {
		if auto {
			// In auto mode (Git hook), don't fail the Git operation.
			fmt.Fprintf(os.Stderr, "snap: auto-save warning: %v\n", err)
			return
		}
		fatal(err)
	}
}

func cmdRestore(ctx context.Context) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "snap: restore requires a git hash argument")
		os.Exit(1)
	}

	root, err := findRepoRoot()
	if err != nil {
		fatal(err)
	}

	gitHash := os.Args[2]
	orch := orchestrator.New(root, snap.Registry)
	if err := orch.Restore(ctx, gitHash); err != nil {
		fatal(err)
	}
}

func cmdVerify(ctx context.Context) {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "snap: verify requires a git hash argument")
		os.Exit(1)
	}

	root, err := findRepoRoot()
	if err != nil {
		fatal(err)
	}

	gitHash := os.Args[2]
	orch := orchestrator.New(root, snap.Registry)
	if err := orch.VerifyAll(ctx, gitHash); err != nil {
		fatal(err)
	}
}

// findRepoRoot walks up from cwd to find a directory containing .git/.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("snap: getwd: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("snap: not a git repository (no .git found)")
}

func fatal(err error) {
	msg := err.Error()
	if !strings.HasPrefix(msg, "snap:") {
		msg = "snap: " + msg
	}
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
