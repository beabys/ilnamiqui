package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// FindProjectRoot walks up from CWD looking for a .opencode/ directory.
// Returns the absolute path containing .opencode/, or an error if not found.
func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}

	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("abs: %w", err)
	}

	for {
		info, err := os.Stat(filepath.Join(dir, ".opencode"))
		if err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf(
				"no .opencode directory found from %s to root: not in an opencode project",
				dir,
			)
		}
		dir = parent
	}
}

// DBPath returns the absolute path to the project's SQLite database.
func DBPath() (string, error) {
	root, err := FindProjectRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".opencode", "ilnamiqui.db"), nil
}

// ProjectSlug returns a short hex hash identifying the project root.
func ProjectSlug() (string, error) {
	root, err := FindProjectRoot()
	if err != nil {
		return "", err
	}
	h := sha256.Sum256([]byte(root))
	return fmt.Sprintf("%x", h[:8]), nil
}
