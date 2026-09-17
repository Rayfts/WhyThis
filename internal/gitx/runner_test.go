package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileModeDetectsSubmoduleEntry(t *testing.T) {
	dir := t.TempDir()
	gitRunnerT(t, dir, "init", "-q")
	gitRunnerT(t, dir, "config", "user.name", "Fixture")
	gitRunnerT(t, dir, "config", "user.email", "fixture@example.com")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRunnerT(t, dir, "add", "base.txt")
	gitRunnerT(t, dir, "commit", "-q", "-m", "base")
	sha := strings.TrimSpace(gitRunnerTOut(t, dir, "rev-parse", "HEAD"))
	gitRunnerT(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+sha+",deps/example")
	g, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	mode, err := g.FileMode(context.Background(), "deps/example")
	if err != nil {
		t.Fatal(err)
	}
	if mode != "160000" {
		t.Fatalf("mode=%q want 160000", mode)
	}
}

func gitRunnerT(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
func gitRunnerTOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}
