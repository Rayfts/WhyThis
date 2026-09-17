package history

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rayfts/WhyThis/internal/gitx"
)

func TestSyntheticArchaeologyHistory(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "WhyThis Fixture")
	git(t, dir, "config", "user.email", "fixture@example.com")
	write(t, dir, "worker.go", "package fixture\n\nfunc retry() int { return 1 }\n")
	git(t, dir, "add", "worker.go")
	git(t, dir, "commit", "-q", "-m", "feat: add retry worker")
	write(t, dir, "worker.go", "package fixture\n\nfunc retry() int {\n\t// avoid retry storm after timeout\n\treturn 2\n}\n")
	git(t, dir, "add", "worker.go")
	git(t, dir, "commit", "-q", "-m", "fix: cap retry after timeout refs #12")
	write(t, dir, "worker_test.go", "package fixture\n\nfunc TestRetryRegression() {}\n")
	git(t, dir, "add", "worker_test.go")
	git(t, dir, "commit", "-q", "-m", "test: cover retry regression")
	git(t, dir, "mv", "worker.go", "runner.go")
	git(t, dir, "commit", "-q", "-m", "refactor: rename worker to runner")
	g, err := gitx.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &Archaeologist{Git: g}
	rep, err := a.File(context.Background(), "runner.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Timeline) < 3 {
		t.Fatalf("expected history across rename, got %d commits", len(rep.Timeline))
	}
	joined := ""
	for _, f := range rep.Facts {
		joined += f.Text + "\n"
	}
	if !strings.Contains(joined, "worker.go → runner.go") {
		t.Fatalf("rename provenance missing:\n%s", joined)
	}
	line, err := a.Line(context.Background(), Target{Path: "runner.go", Start: 4, End: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(line.Facts) == 0 || len(line.Graph.Nodes) == 0 {
		t.Fatal("expected blame-backed evidence")
	}
}

func BenchmarkFileHistorySynthetic(b *testing.B) {
	dir := b.TempDir()
	gitB(b, dir, "init", "-q")
	gitB(b, dir, "config", "user.name", "Bench")
	gitB(b, dir, "config", "user.email", "bench@example.com")
	for i := 0; i < 40; i++ {
		_ = os.WriteFile(filepath.Join(dir, "f.go"), []byte("package p\n// revision\n"+strings.Repeat("x", i+1)+"\n"), 0o644)
		gitB(b, dir, "add", "f.go")
		gitB(b, dir, "commit", "-q", "-m", "change history")
	}
	g, _ := gitx.New(dir)
	a := &Archaeologist{Git: g}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := a.File(context.Background(), "f.go"); err != nil {
			b.Fatal(err)
		}
	}
}
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
func gitB(b *testing.B, dir string, args ...string) {
	b.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	if out, err := c.CombinedOutput(); err != nil {
		b.Fatalf("git %v: %v: %s", args, err, out)
	}
}
func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
