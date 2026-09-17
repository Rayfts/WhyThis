package graph

import (
	"fmt"

	"github.com/Rayfts/WhyThis/pkg/evidence"
)

// Validate verifies referential integrity for an evidence graph. Every edge
// endpoint and every claim evidence ID is expected to be backed by a node in
// the same graph before the report is persisted or exported.
func Validate(g evidence.Graph) error {
	nodes := make(map[string]struct{}, len(g.Nodes))
	for _, n := range g.Nodes {
		if n.ID == "" {
			return fmt.Errorf("graph contains node with empty id")
		}
		if _, exists := nodes[n.ID]; exists {
			return fmt.Errorf("graph contains duplicate node id %q", n.ID)
		}
		nodes[n.ID] = struct{}{}
	}
	for _, e := range g.Edges {
		if _, ok := nodes[e.From]; !ok {
			return fmt.Errorf("edge %q -> %q (%s) has missing source node", e.From, e.To, e.Kind)
		}
		if _, ok := nodes[e.To]; !ok {
			return fmt.Errorf("edge %q -> %q (%s) has missing target node", e.From, e.To, e.Kind)
		}
	}
	return nil
}
