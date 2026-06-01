package config

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTempProject creates a temp dir with a .ilnamiqui/ subdirectory and returns its real path.
func setupTempProject(t *testing.T) string {
	t.Helper()
	rawDir := t.TempDir()
	// Resolve symlinks (macOS /var -> /private/var) so path matches os.Getwd().
	dir, err := filepath.EvalSymlinks(rawDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir+"/.ilnamiqui", 0o755); err != nil {
		t.Fatal(err)
	}
	// Write sentinel so project appears initialized
	if err := os.WriteFile(dir+"/.ilnamiqui/.initialized", []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// setupLegacyProject creates a temp dir with .opencode/ilnamiqui.db (empty file).
func setupLegacyProject(t *testing.T) string {
	t.Helper()
	rawDir := t.TempDir()
	dir, err := filepath.EvalSymlinks(rawDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir+"/.opencode", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/.opencode/ilnamiqui.db", []byte("legacy db"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFindProjectRoot_found(t *testing.T) {
	dir := setupTempProject(t)

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	root, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if root != dir {
		t.Fatalf("expected root %q, got %q", dir, root)
	}
}

func TestFindProjectRoot_fromSubdir(t *testing.T) {
	dir := setupTempProject(t)

	subDir := dir + "/a/b/c"
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	if err := os.Chdir(subDir); err != nil {
		t.Fatal(err)
	}

	root, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if root != dir {
		t.Fatalf("expected root %q, got %q", dir, root)
	}
}

func resolveTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFindProjectRoot_notFound(t *testing.T) {
	dir := resolveTempDir(t) // no .ilnamiqui

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	_, err = FindProjectRoot()
	if err == nil {
		t.Fatal("expected error for project without .ilnamiqui")
	}
}

func TestFindProjectRoot_symlink(t *testing.T) {
	dir := setupTempProject(t)

	// Remove the real .ilnamiqui directory
	if err := os.RemoveAll(dir + "/.ilnamiqui"); err != nil {
		t.Fatal(err)
	}
	// Create a real target directory
	target := dir + "/ilnamiqui-real"
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create .ilnamiqui as a symlink to the real target
	if err := os.Symlink(target, dir+"/.ilnamiqui"); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	root, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("expected no error with symlink, got %v", err)
	}
	if root != dir {
		t.Fatalf("expected root %q, got %q", dir, root)
	}
}

func TestDBPath(t *testing.T) {
	dir := setupTempProject(t)

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	p, err := DBPath()
	if err != nil {
		t.Fatalf("DBPath() error: %v", err)
	}
	expected := filepath.Join(dir, ".ilnamiqui", "ilnamiqui.db")
	if p != expected {
		t.Fatalf("expected path %q, got %q", expected, p)
	}
}

func TestDBPath_noProject(t *testing.T) {
	dir := resolveTempDir(t)

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	_, err = DBPath()
	if err == nil {
		t.Fatal("expected error for DBPath without .ilnamiqui")
	}
}

func TestProjectSlug(t *testing.T) {
	dir := setupTempProject(t)

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	slug, err := ProjectSlug()
	if err != nil {
		t.Fatalf("ProjectSlug() error: %v", err)
	}
	if slug == "" {
		t.Fatal("expected non-empty slug")
	}
	if len(slug) != 16 { // 8 bytes → 16 hex chars
		t.Fatalf("expected 16-char hex slug, got %q (len=%d)", slug, len(slug))
	}
}

func TestProjectSlug_stable(t *testing.T) {
	dir := setupTempProject(t)

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	slug1, err := ProjectSlug()
	if err != nil {
		t.Fatal(err)
	}
	slug2, err := ProjectSlug()
	if err != nil {
		t.Fatal(err)
	}
	if slug1 != slug2 {
		t.Fatalf("slug should be stable: %q != %q", slug1, slug2)
	}
}

func TestProjectSlug_noProject(t *testing.T) {
	dir := resolveTempDir(t)

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	_, err = ProjectSlug()
	if err == nil {
		t.Fatal("expected error for ProjectSlug without .ilnamiqui")
	}
}

func TestFindProjectRoot_rejectsFile(t *testing.T) {
	dir := resolveTempDir(t)
	// Create a file named .ilnamiqui (not a directory)
	if err := os.WriteFile(dir+"/.ilnamiqui", []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	_, err = FindProjectRoot()
	if err == nil {
		t.Fatal("expected error when .ilnamiqui is a file, not directory")
	}
}

// --- Migration tests ---

func TestNeedsMigration_true(t *testing.T) {
	dir := setupLegacyProject(t)

	if !NeedsMigration(dir) {
		t.Fatal("NeedsMigration should be true for legacy project with .opencode/ilnamiqui.db")
	}
}

func TestNeedsMigration_false_emptyProject(t *testing.T) {
	dir := resolveTempDir(t) // no .opencode/

	if NeedsMigration(dir) {
		t.Fatal("NeedsMigration should be false for empty project")
	}
}

func TestNeedsMigration_false_migratedAndLegacy(t *testing.T) {
	dir := setupLegacyProject(t) // has .opencode/ilnamiqui.db

	// Also create .ilnamiqui/ dir (migrated)
	if err := os.MkdirAll(dir+"/.ilnamiqui", 0o755); err != nil {
		t.Fatal(err)
	}

	if !NeedsMigration(dir) {
		t.Fatal("NeedsMigration should be true when legacy DB still exists")
	}
}

func TestMigrateLegacy_success(t *testing.T) {
	dir := setupLegacyProject(t)

	if err := MigrateLegacy(dir); err != nil {
		t.Fatalf("MigrateLegacy error: %v", err)
	}

	// Verify .ilnamiqui/ilnamiqui.db exists
	if _, err := os.Stat(dir + "/.ilnamiqui/ilnamiqui.db"); os.IsNotExist(err) {
		t.Fatal(".ilnamiqui/ilnamiqui.db not found after migration")
	}

	// Verify sentinel exists
	if _, err := os.Stat(dir + "/.ilnamiqui/.initialized"); os.IsNotExist(err) {
		t.Fatal(".ilnamiqui/.initialized not found after migration")
	}

	// Verify legacy DB no longer exists at old location
	if _, err := os.Stat(dir + "/.opencode/ilnamiqui.db"); err == nil {
		t.Fatal("legacy .opencode/ilnamiqui.db should not exist after migration")
	}

	// Verify IsInitialized now returns true
	if !IsInitialized(dir) {
		t.Fatal("IsInitialized should return true after migration")
	}
}

func TestMigrateLegacy_idempotent(t *testing.T) {
	dir := setupLegacyProject(t)

	if err := MigrateLegacy(dir); err != nil {
		t.Fatalf("first MigrateLegacy error: %v", err)
	}

	// Second call — should be idempotent
	if err := MigrateLegacy(dir); err != nil {
		t.Fatalf("second MigrateLegacy (idempotent) error: %v", err)
	}

	// Final state consistent
	if _, err := os.Stat(dir + "/.ilnamiqui/ilnamiqui.db"); os.IsNotExist(err) {
		t.Fatal(".ilnamiqui/ilnamiqui.db not found after second migration")
	}
	if _, err := os.Stat(dir + "/.ilnamiqui/.initialized"); os.IsNotExist(err) {
		t.Fatal(".ilnamiqui/.initialized not found after second migration")
	}
}

func TestMigrateLegacy_noLegacySource(t *testing.T) {
	dir := resolveTempDir(t) // no .opencode/ dir at all

	// Should return nil (no-op)
	if err := MigrateLegacy(dir); err != nil {
		t.Fatalf("MigrateLegacy on empty dir should succeed: %v", err)
	}
}

func TestIsInitialized_true(t *testing.T) {
	dir := setupTempProject(t) // has .ilnamiqui/.initialized

	if !IsInitialized(dir) {
		t.Fatal("IsInitialized should be true for initialized project")
	}
}

func TestIsInitialized_false_emptyDir(t *testing.T) {
	dir := resolveTempDir(t) // no .ilnamiqui/ at all

	if IsInitialized(dir) {
		t.Fatal("IsInitialized should be false for empty dir")
	}
}

func TestIsInitialized_false_noSentinel(t *testing.T) {
	dir := resolveTempDir(t)
	if err := os.MkdirAll(dir+"/.ilnamiqui", 0o755); err != nil {
		t.Fatal(err)
	}
	// No sentinel file

	if IsInitialized(dir) {
		t.Fatal("IsInitialized should be false when sentinel missing")
	}
}

func TestWriteSentinel(t *testing.T) {
	dir := resolveTempDir(t)
	if err := os.MkdirAll(dir+"/.ilnamiqui", 0o755); err != nil {
		t.Fatal(err)
	}

	if err := WriteSentinel(dir); err != nil {
		t.Fatalf("WriteSentinel error: %v", err)
	}

	if _, err := os.Stat(dir + "/.ilnamiqui/.initialized"); os.IsNotExist(err) {
		t.Fatal("sentinel file not created by WriteSentinel")
	}
}
