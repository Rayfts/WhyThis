package app

import (
	"strings"
	"testing"

	"github.com/Rayfts/WhyThis/internal/githubx"
	"github.com/Rayfts/WhyThis/internal/graph"
	"github.com/Rayfts/WhyThis/pkg/evidence"
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

func TestMergeHistoricalReportKeepsSupportingFileNodes(t *testing.T) {
	b := graph.New()
	b.AddNode(evidence.Node{ID: "pr:7", Kind: evidence.KindPullRequest})
	rep := evidence.Report{Graph: evidence.Graph{
		Nodes: []evidence.Node{
			{ID: "file:new.go", Kind: evidence.KindFile},
			{ID: "file:old.go", Kind: evidence.KindFile},
			{ID: "commit:abc", Kind: evidence.KindCommit, Attributes: map[string]any{"sha": "abc"}},
		},
		Edges: []evidence.Edge{
			{From: "file:new.go", To: "file:old.go", Kind: evidence.EdgeRenamedFrom},
			{From: "file:new.go", To: "commit:abc", Kind: evidence.EdgeChangedBy},
		},
	}}
	var facts []evidence.Claim
	var timeline []evidence.Node
	mergeHistoricalReport(b, &facts, &timeline, rep, nil)
	if err := graph.Validate(b.Build()); err != nil {
		t.Fatalf("merged PR history graph is not closed: %v", err)
	}
}
