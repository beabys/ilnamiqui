package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

const (
	ILNAMIQUI_DIR = ".ilnamiqui"
	ILNAMIQUI_DB  = "ilnamiqui.db"
	SENTINEL_FILE = ".initialized"
)

const (
	LEGACY_OPENCODE_DIR = ".opencode"
	LEGACY_DB_NAME      = "ilnamiqui.db"
)

// FindProjectRoot walks up from CWD looking for a .ilnamiqui/ directory.
// Returns the absolute path containing .ilnamiqui/, or an error if not found.
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
		info, err := os.Stat(filepath.Join(dir, ILNAMIQUI_DIR))
		if err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf(
				"no %s directory found from %s to root: not an ilnamiqui project",
				ILNAMIQUI_DIR,
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
	return filepath.Join(root, ILNAMIQUI_DIR, ILNAMIQUI_DB), nil
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

// NeedsMigration checks if legacy .opencode/ilnamiqui.db exists.
func NeedsMigration(root string) bool {
	legacyDB := filepath.Join(root, LEGACY_OPENCODE_DIR, LEGACY_DB_NAME)
	info, err := os.Stat(legacyDB)
	return err == nil && !info.IsDir()
}

// IsInitialized checks if sentinel file exists — fast path.
func IsInitialized(root string) bool {
	sentinel := filepath.Join(root, ILNAMIQUI_DIR, SENTINEL_FILE)
	_, err := os.Stat(sentinel)
	return err == nil
}

// MigrateLegacy moves .opencode/ilnamiqui.db → .ilnamiqui/ilnamiqui.db.
// Idempotent — safe to call multiple times.
func MigrateLegacy(root string) error {
	legacyDB := filepath.Join(root, LEGACY_OPENCODE_DIR, LEGACY_DB_NAME)
	ilnamiquiDir := filepath.Join(root, ILNAMIQUI_DIR)
	newDB := filepath.Join(ilnamiquiDir, ILNAMIQUI_DB)
	sentinel := filepath.Join(ilnamiquiDir, SENTINEL_FILE)

	// Already migrated or no legacy source
	if _, err := os.Stat(legacyDB); os.IsNotExist(err) {
		return nil
	}

	// Create .ilnamiqui/ dir
	if err := os.MkdirAll(ilnamiquiDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", ILNAMIQUI_DIR, err)
	}

	// Move DB
	if err := os.Rename(legacyDB, newDB); err != nil {
		return fmt.Errorf("migrate db: %w", err)
	}

	// Write sentinel
	if err := os.WriteFile(sentinel, []byte{}, 0o644); err != nil {
		return fmt.Errorf("write sentinel: %w", err)
	}

	return nil
}

// WriteSentinel writes the sentinel file marking the project as initialized.
func WriteSentinel(root string) error {
	sentinel := filepath.Join(root, ILNAMIQUI_DIR, SENTINEL_FILE)
	return os.WriteFile(sentinel, []byte{}, 0o644)
}
