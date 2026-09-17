package history

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rayfts/WhyThis/internal/gitx"
	"github.com/Rayfts/WhyThis/internal/graph"
)

func TestSymbolUsesGoASTAnchor(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "WhyThis Fixture")
	git(t, dir, "config", "user.email", "fixture@example.com")
	write(t, dir, "service.go", "package fixture\ntype Service struct{}\nfunc (s *Service) Run() int { return 1 }\n")
	git(t, dir, "add", "service.go")
	git(t, dir, "commit", "-q", "-m", "feat: add service run")
	g, err := gitx.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := (&Archaeologist{Git: g}).Symbol(context.Background(), "Service.Run")
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, c := range rep.Facts {
		joined += c.Text + "\n"
	}
	if !strings.Contains(joined, "Go AST anchors Service.Run") {
		t.Fatalf("missing AST fact:\n%s", joined)
	}
	if err := graph.Validate(rep.Graph); err != nil {
		t.Fatalf("symbol graph not closed: %v", err)
	}
}

func TestDeletedFileHistoryRemainsQueryable(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "WhyThis Fixture")
	git(t, dir, "config", "user.email", "fixture@example.com")
	write(t, dir, "gone.go", "package fixture\nfunc Gone() {}\n")
	git(t, dir, "add", "gone.go")
	git(t, dir, "commit", "-q", "-m", "feat: add temporary implementation")
	git(t, dir, "rm", "-q", "gone.go")
	git(t, dir, "commit", "-q", "-m", "refactor: remove obsolete implementation")
	g, err := gitx.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := (&Archaeologist{Git: g}).File(context.Background(), "gone.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Timeline) < 2 {
		t.Fatalf("expected add/remove history for deleted path, got %d", len(rep.Timeline))
	}
	if err := graph.Validate(rep.Graph); err != nil {
		t.Fatalf("deleted-file graph not closed: %v", err)
	}
}

func TestMergeCommitPreservesMultipleParents(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "WhyThis Fixture")
	git(t, dir, "config", "user.email", "fixture@example.com")
	write(t, dir, "base.go", "package fixture\n")
	git(t, dir, "add", "base.go")
	git(t, dir, "commit", "-q", "-m", "base")
	git(t, dir, "checkout", "-q", "-b", "feature")
	write(t, dir, "feature.go", "package fixture\nfunc Feature() {}\n")
	git(t, dir, "add", "feature.go")
	git(t, dir, "commit", "-q", "-m", "feature")
	git(t, dir, "checkout", "-q", "master")
	write(t, dir, "main.go", "package fixture\nfunc MainOnly() {}\n")
	git(t, dir, "add", "main.go")
	git(t, dir, "commit", "-q", "-m", "main change")
	git(t, dir, "merge", "-q", "--no-ff", "feature", "-m", "merge feature")
	mergeSHA := gitOutput(t, dir, "rev-parse", "HEAD")
	g, err := gitx.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := (&Archaeologist{Git: g}).Commit(context.Background(), mergeSHA)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Timeline) != 1 {
		t.Fatalf("expected one merge commit node")
	}
	parents, ok := rep.Timeline[0].Attributes["parents"].([]string)
	if !ok || len(parents) != 2 {
		t.Fatalf("expected two merge parents, got %#v", rep.Timeline[0].Attributes["parents"])
	}
}

func TestCommitExpandsExplicitRevertChain(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.name", "WhyThis Fixture")
	git(t, dir, "config", "user.email", "fixture@example.com")
	write(t, dir, "retry.go", "package fixture\nfunc Retry() int { return 1 }\n")
	git(t, dir, "add", "retry.go")
	git(t, dir, "commit", "-q", "-m", "feat: initial retry")
	write(t, dir, "retry.go", "package fixture\nfunc Retry() int { return 2 }\n")
	git(t, dir, "add", "retry.go")
	git(t, dir, "commit", "-q", "-m", "fix: bound retry regression")
	fix := gitOutput(t, dir, "rev-parse", "HEAD")
	git(t, dir, "revert", "--no-edit", fix)
	firstRevert := gitOutput(t, dir, "rev-parse", "HEAD")
	git(t, dir, "revert", "--no-edit", firstRevert)
	secondRevert := gitOutput(t, dir, "rev-parse", "HEAD")

	g, err := gitx.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := (&Archaeologist{Git: g}).Commit(context.Background(), secondRevert)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Timeline) < 3 {
		t.Fatalf("expected expanded revert chain, got %d commits", len(rep.Timeline))
	}
	joined := ""
	for _, c := range rep.Facts {
		joined += c.Text + "\n"
	}
	if !strings.Contains(joined, "2-link revert chain") {
		t.Fatalf("missing revert-chain fact:\n%s", joined)
	}
	if err := graph.Validate(rep.Graph); err != nil {
		t.Fatalf("revert-chain graph not closed: %v", err)
	}
}

func TestShallowHistoryIsExplicitlyUnknown(t *testing.T) {
	source := t.TempDir()
	git(t, source, "init", "-q")
	git(t, source, "config", "user.name", "WhyThis Fixture")
	git(t, source, "config", "user.email", "fixture@example.com")
	write(t, source, "x.go", "package fixture\n// one\n")
	git(t, source, "add", "x.go")
	git(t, source, "commit", "-q", "-m", "first")
	write(t, source, "x.go", "package fixture\n// two\n")
	git(t, source, "add", "x.go")
	git(t, source, "commit", "-q", "-m", "second")

	clone := filepath.Join(t.TempDir(), "shallow")
	c := exec.Command("git", "clone", "-q", "--depth=1", "file://"+source, clone)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("shallow clone: %v: %s", err, out)
	}
	g, err := gitx.New(clone)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := (&Archaeologist{Git: g}).File(context.Background(), "x.go")
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, c := range rep.Unknowns {
		joined += c.Text + "\n"
	}
	if !strings.Contains(joined, "shallow clone") {
		t.Fatalf("expected explicit shallow-history unknown:\n%s", joined)
	}
}
