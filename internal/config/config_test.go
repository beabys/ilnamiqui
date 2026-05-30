package config

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTempProject creates a temp dir with a .opencode/ subdirectory and returns its real path.
func setupTempProject(t *testing.T) string {
	t.Helper()
	rawDir := t.TempDir()
	// Resolve symlinks (macOS /var -> /private/var) so path matches os.Getwd().
	dir, err := filepath.EvalSymlinks(rawDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir+"/.opencode", 0o755); err != nil {
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
	dir := resolveTempDir(t) // no .opencode

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
		t.Fatal("expected error for project without .opencode")
	}
}

func TestFindProjectRoot_symlink(t *testing.T) {
	dir := setupTempProject(t)

	// Remove the real .opencode directory
	if err := os.RemoveAll(dir + "/.opencode"); err != nil {
		t.Fatal(err)
	}
	// Create a real target directory
	target := dir + "/opencode-real"
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create .opencode as a symlink to the real target
	if err := os.Symlink(target, dir+"/.opencode"); err != nil {
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
	expected := filepath.Join(dir, ".opencode", "ilnamiqui.db")
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
		t.Fatal("expected error for DBPath without .opencode")
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
		t.Fatal("expected error for ProjectSlug without .opencode")
	}
}

func TestFindProjectRoot_rejectsFile(t *testing.T) {
	dir := resolveTempDir(t)
	// Create a file named .opencode (not a directory)
	if err := os.WriteFile(dir+"/.opencode", []byte("not a dir"), 0o644); err != nil {
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
		t.Fatal("expected error when .opencode is a file, not directory")
	}
}
