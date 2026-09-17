package symbol

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveGoFunctionMethodAndPath(t *testing.T) {
	d := t.TempDir()
	src := "package demo\ntype Service struct{}\nfunc Hello() {}\nfunc (s *Service) Run() {}\n"
	if err := os.WriteFile(filepath.Join(d, "demo.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	locs, err := ResolveGo(d, "Service.Run")
	if err != nil {
		t.Fatal(err)
	}
	if len(locs) != 1 || locs[0].Receiver != "Service" || locs[0].Start != 4 {
		t.Fatalf("unexpected: %+v", locs)
	}
	locs, err = ResolveGo(d, "demo.go::Hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(locs) != 1 || locs[0].Name != "Hello" {
		t.Fatalf("unexpected: %+v", locs)
	}
}
