package ownership

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestResolveUsesGitHubLocationPrecedenceAndLastMatch(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CODEOWNERS"), []byte("*.go @root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".github"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "* @global\ninternal/* @internal\ninternal/secure.go @security @reviewer\n"
	if err := os.WriteFile(filepath.Join(root, ".github", "CODEOWNERS"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(root, "internal/secure.go")
	if err != nil {
		t.Fatal(err)
	}
	if got.File != ".github/CODEOWNERS" || got.Pattern != "internal/secure.go" || got.Line != 3 || !got.Matched {
		t.Fatalf("unexpected match: %#v", got)
	}
	if want := []string{"@security", "@reviewer"}; !reflect.DeepEqual(got.Owners, want) {
		t.Fatalf("owners=%v want=%v", got.Owners, want)
	}
}

func TestResolveReportsConfiguredButUnmatched(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "CODEOWNERS"), []byte("docs/* @docs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(root, "src/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Configured || got.Matched || len(got.Owners) != 0 {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestResolveWithoutCODEOWNERS(t *testing.T) {
	got, err := Resolve(t.TempDir(), "main.go")
	if err != nil {
		t.Fatal(err)
	}
	if got.Configured || got.Matched {
		t.Fatalf("unexpected result: %#v", got)
	}
}
