package gitx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func BenchmarkRecentCommitPathsSynthetic(b *testing.B) {
	dir := b.TempDir()
	gitRunnerB(b, dir, "init", "-q")
	gitRunnerB(b, dir, "config", "user.name", "Bench")
	gitRunnerB(b, dir, "config", "user.email", "bench@example.com")
	for i := 0; i < 200; i++ {
		name := filepath.Join(dir, fmt.Sprintf("pkg/p%d/f.go", i%40))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(fmt.Sprintf("package p\n// %d\n", i)), 0o644); err != nil {
			b.Fatal(err)
		}
		gitRunnerB(b, dir, "add", ".")
		gitRunnerB(b, dir, "commit", "-q", "-m", "change")
	}
	g, err := New(dir)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		paths, order, err := g.RecentCommitPaths(context.Background(), 200)
		if err != nil {
			b.Fatal(err)
		}
		if len(paths) == 0 || len(order) == 0 {
			b.Fatal("empty traversal")
		}
	}
}

func gitRunnerB(b *testing.B, dir string, args ...string) {
	b.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		b.Fatalf("git %v: %v: %s", args, err, out)
	}
}
