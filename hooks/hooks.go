// Package hooks manages Git hook installation and chaining.
// Snap installs post-commit and post-checkout hooks to maintain
// automatic code↔state synchronization.
package hooks

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	postCommitScript = `#!/bin/sh
# Installed by snap init — do not edit.
# Captures system state on every commit.
snap save --auto
# Chain to the original hook if it exists.
if [ -x "$(dirname "$0")/post-commit.user" ]; then
    exec "$(dirname "$0")/post-commit.user" "$@"
fi
`

	postCheckoutScript = `#!/bin/sh
# Installed by snap init — do not edit.
# Restores state when switching commits.
PREV_HEAD="$1"
NEW_HEAD="$2"

if [ "$PREV_HEAD" != "$NEW_HEAD" ]; then
    snap restore "$NEW_HEAD"
fi
# Chain to the original hook if it exists.
if [ -x "$(dirname "$0")/post-checkout.user" ]; then
    exec "$(dirname "$0")/post-checkout.user" "$@"
fi
`
)

// Install installs Snap's Git hooks into the given .git/hooks directory.
// Existing hooks are preserved by renaming them with a .user suffix.
func Install(gitHooksDir string) error {
	hooks := map[string]string{
		"post-commit":   postCommitScript,
		"post-checkout": postCheckoutScript,
	}

	for name, script := range hooks {
		hookPath := filepath.Join(gitHooksDir, name)
		userPath := filepath.Join(gitHooksDir, name+".user")

		// Chain existing hook if present.
		if info, err := os.Stat(hookPath); err == nil && info.Mode().IsRegular() {
			// Check if it's already ours. Simple heuristic: we check for our marker.
			data, readErr := os.ReadFile(hookPath)
			if readErr == nil && contains(string(data), "Installed by snap init") {
				// Already installed. Skip.
				continue
			}
			// Rename the existing hook for chaining.
			if err := os.Rename(hookPath, userPath); err != nil {
				return fmt.Errorf("hooks: preserve existing %s: %w", name, err)
			}
		}

		if err := os.WriteFile(hookPath, []byte(script), 0o755); err != nil {
			return fmt.Errorf("hooks: write %s: %w", name, err)
		}
	}

	return nil
}

// contains reports whether s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
