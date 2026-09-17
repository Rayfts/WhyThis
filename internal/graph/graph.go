package graph

import (
	"fmt"
	"sync"

	"github.com/Rayfts/WhyThis/pkg/evidence"
)

type Builder struct {
	mu    sync.Mutex
	nodes map[string]evidence.Node
	edges map[string]evidence.Edge
}

func New() *Builder {
	return &Builder{nodes: map[string]evidence.Node{}, edges: map[string]evidence.Edge{}}
}

func (b *Builder) AddNode(n evidence.Node) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nodes[n.ID] = n
}

func (b *Builder) AddEdge(e evidence.Edge) {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := fmt.Sprintf("%s|%s|%s", e.From, e.Kind, e.To)
	b.edges[key] = e
}

func (b *Builder) Build() evidence.Graph {
	b.mu.Lock()
	defer b.mu.Unlock()
	g := evidence.Graph{Nodes: make([]evidence.Node, 0, len(b.nodes)), Edges: make([]evidence.Edge, 0, len(b.edges))}
	for _, n := range b.nodes {
		g.Nodes = append(g.Nodes, n)
	}
	for _, e := range b.edges {
		g.Edges = append(g.Edges, e)
	}
	return g
}
