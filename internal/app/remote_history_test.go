package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rayfts/WhyThis/internal/gitx"
	"github.com/Rayfts/WhyThis/internal/graph"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

func TestCommitRemoteHistoryAddsCIFailureAndRelease(t *testing.T) {
	dir := t.TempDir()
	gitApp(t, dir, "init", "-q")
	gitApp(t, dir, "config", "user.name", "Fixture")
	gitApp(t, dir, "config", "user.email", "fixture@example.com")
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitApp(t, dir, "add", "x.go")
	gitApp(t, dir, "commit", "-q", "-m", "initial")
	gitApp(t, dir, "remote", "add", "origin", "https://github.com/acme/demo.git")
	gitApp(t, dir, "tag", "v1.0.0")
	sha := strings.TrimSpace(gitAppOut(t, dir, "rev-parse", "HEAD"))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/check-runs"):
			_, _ = fmt.Fprint(w, `{"check_runs":[{"id":7,"name":"linux","status":"completed","conclusion":"failure","html_url":"https://example/check"}]}`)
		case strings.Contains(r.URL.Path, "/statuses"):
			_, _ = fmt.Fprint(w, `[]`)
		case strings.HasSuffix(r.URL.Path, "/releases"):
			_, _ = fmt.Fprint(w, `[{"id":9,"tag_name":"v1.0.0","name":"v1.0.0","html_url":"https://example/release"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	g, err := gitx.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(g)
	s.ConfigureGitHub("test-token", srv.URL, nil)
	rep, err := s.Commit(context.Background(), sha)
	if err != nil {
		t.Fatal(err)
	}
	foundCI, foundRelease := false, false
	lastID := ""
	for _, n := range rep.Graph.Nodes {
		if lastID != "" && n.ID < lastID {
			t.Fatalf("remote enrichment nodes are not deterministic: %q before %q", lastID, n.ID)
		}
		lastID = n.ID
		if n.Kind == evidence.KindCIFailure && n.Label == "linux" {
			foundCI = true
		}
		if n.Kind == evidence.KindRelease && n.Label == "v1.0.0" {
			foundRelease = true
		}
	}
	if !foundCI || !foundRelease {
		t.Fatalf("expected CI failure and release nodes: %+v", rep.Graph.Nodes)
	}
	if err := graph.Validate(rep.Graph); err != nil {
		t.Fatalf("remote history graph not closed: %v", err)
	}
}

func gitApp(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
func gitAppOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
