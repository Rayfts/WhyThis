package graph

import (
	"testing"

	"github.com/Rayfts/WhyThis/pkg/evidence"
)

func TestValidateRejectsDanglingEdge(t *testing.T) {
	g := evidence.Graph{
		Nodes: []evidence.Node{{ID: "a"}},
		Edges: []evidence.Edge{{From: "a", To: "missing", Kind: evidence.EdgeReferences}},
	}
	if err := Validate(g); err == nil {
		t.Fatal("expected dangling edge to be rejected")
	}
}

func TestValidateAcceptsClosedGraph(t *testing.T) {
	g := evidence.Graph{
		Nodes: []evidence.Node{{ID: "a"}, {ID: "b"}},
		Edges: []evidence.Edge{{From: "a", To: "b", Kind: evidence.EdgeReferences}},
	}
	if err := Validate(g); err != nil {
		t.Fatal(err)
	}
}
