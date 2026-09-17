package index

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Rayfts/WhyThis/internal/gitx"
	"github.com/Rayfts/WhyThis/internal/storage"
)

func BenchmarkIndexerIncremental(b *testing.B) {
	dir := b.TempDir()
	gitIndexB(b, dir, "init", "-q")
	gitIndexB(b, dir, "config", "user.name", "Bench")
	gitIndexB(b, dir, "config", "user.email", "bench@example.com")
	for i := 0; i < 100; i++ {
		if err := os.WriteFile(filepath.Join(dir, "f.go"), []byte(fmt.Sprintf("package p\n// %d\n", i)), 0o644); err != nil {
			b.Fatal(err)
		}
		gitIndexB(b, dir, "add", "f.go")
		gitIndexB(b, dir, "commit", "-q", "-m", fmt.Sprintf("change %d", i))
	}
	g, err := gitx.New(dir)
	if err != nil {
		b.Fatal(err)
	}
	st, err := storage.Open(filepath.Join(dir, ".whyth", "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	idx := &Indexer{Git: g, Store: st}
	if _, err := idx.Run(context.Background()); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		if err := os.WriteFile(filepath.Join(dir, "f.go"), []byte(fmt.Sprintf("package p\n// incremental %d\n", i)), 0o644); err != nil {
			b.Fatal(err)
		}
		gitIndexB(b, dir, "add", "f.go")
		gitIndexB(b, dir, "commit", "-q", "-m", "incremental")
		b.StartTimer()
		if _, err := idx.Run(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}

func gitIndexB(b *testing.B, dir string, args ...string) {
	b.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		b.Fatalf("git %v: %v: %s", args, err, out)
	}
}
