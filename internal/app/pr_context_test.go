package app

import (
	"strings"
	"testing"

	"github.com/Rayfts/WhyThis/internal/githubx"
)

func TestSelectPRHistoryFilesBoundsAndFiltersNoise(t *testing.T) {
	files := []githubx.PRFile{
		{Filename: "vendor/a.go", Changes: 999},
		{Filename: "go.sum", Changes: 900},
		{Filename: "src/a.go", Changes: 10},
		{Filename: "src/b.go", Changes: 30},
		{Filename: "src/c.go", Changes: 20},
	}
	selected, skipped := selectPRHistoryFiles(files, 2)
	if skipped != 3 {
		t.Fatalf("skipped=%d, want 3", skipped)
	}
	if len(selected) != 2 || selected[0].Filename != "src/b.go" || selected[1].Filename != "src/c.go" {
		t.Fatalf("unexpected selection: %#v", selected)
	}
}

func TestDiscussionLabelIsBounded(t *testing.T) {
	got := discussionLabel("  a   short   discussion  ")
	if got != "a short discussion" {
		t.Fatalf("label=%q", got)
	}
	long := discussionLabel(strings.Repeat("x", 200))
	if len(long) > 96 {
		t.Fatalf("label length=%d", len(long))
	}
}
