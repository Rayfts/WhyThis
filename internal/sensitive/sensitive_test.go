package sensitive

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeSensitiveAuthPathAndContent(t *testing.T) {
	dir := t.TempDir()
	path := "internal/auth/session.go"
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("package auth\nfunc verify(password string) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := Analyze(dir, path)
	if !r.Sensitive {
		t.Fatal("expected sensitive path")
	}
	foundAccess, foundSecrets := false, false
	for _, f := range r.Findings {
		foundAccess = foundAccess || f.Category == "access-control"
		foundSecrets = foundSecrets || f.Category == "secrets"
	}
	if !foundAccess || !foundSecrets {
		t.Fatalf("expected access-control and secrets findings: %+v", r.Findings)
	}
}

func TestAnalyzeClassifiesGeneratedAndVendor(t *testing.T) {
	if r := Analyze(t.TempDir(), "vendor/acme/generated/client.pb.go"); !r.Generated || !r.Vendor {
		t.Fatalf("expected generated+vendor classification: %+v", r)
	}
}

func TestAnalyzeDoesNotTreatAuthorAsAuth(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "internal", "author", "model.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package author\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := Analyze(d, "internal/author/model.go")
	if r.Sensitive {
		t.Fatalf("author path should not be classified as auth-sensitive: %+v", r.Findings)
	}
}
