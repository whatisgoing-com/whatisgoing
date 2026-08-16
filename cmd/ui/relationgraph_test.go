package main

import (
	"testing"

	"github.com/whatisgoing-com/whatisgoing/internal/ui/coreclient"
)

func TestBuildRelationGraph_EmptyReturnsNoNodes(t *testing.T) {
	g := buildRelationGraph("Elon Musk", nil)
	if len(g.Nodes) != 0 {
		t.Errorf("expected no nodes, got %d", len(g.Nodes))
	}
}

func TestBuildRelationGraph_WeightsEdgeWidthByMaxCooccurrence(t *testing.T) {
	related := []coreclient.RelatedEntity{
		{ID: 1, Name: "Tesla", Type: "ORG", CooccurrenceCount: 10},
		{ID: 2, Name: "SpaceX", Type: "ORG", CooccurrenceCount: 2},
	}
	g := buildRelationGraph("Elon Musk", related)
	if len(g.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(g.Nodes))
	}
	if g.Nodes[0].EdgeWidth <= g.Nodes[1].EdgeWidth {
		t.Errorf("expected the higher-cooccurrence node to have a thicker edge: got %v vs %v", g.Nodes[0].EdgeWidth, g.Nodes[1].EdgeWidth)
	}
}

func TestBuildRelationGraph_ColorsNodesByType(t *testing.T) {
	related := []coreclient.RelatedEntity{
		{ID: 1, Name: "Sam Altman", Type: "PERSON", CooccurrenceCount: 1},
		{ID: 2, Name: "OpenAI", Type: "ORG", CooccurrenceCount: 1},
		{ID: 3, Name: "generative AI", Type: "TOPIC", CooccurrenceCount: 1},
	}
	g := buildRelationGraph("Elon Musk", related)

	want := map[string]string{"PERSON": "fill-blue-500", "ORG": "fill-purple-500", "TOPIC": "fill-amber-500"}
	for _, n := range g.Nodes {
		if got := n.ColorClass; got != want[n.Type] {
			t.Errorf("node %s (%s): color class = %q, want %q", n.Name, n.Type, got, want[n.Type])
		}
	}
}

func TestBuildRelationGraph_NodesStayWithinTheSVGViewport(t *testing.T) {
	related := make([]coreclient.RelatedEntity, 8)
	for i := range related {
		related[i] = coreclient.RelatedEntity{ID: int64(i + 1), Name: "Entity", Type: "ORG", CooccurrenceCount: i + 1}
	}
	g := buildRelationGraph("Center", related)

	for _, n := range g.Nodes {
		if n.X < 0 || n.X > g.Size || n.Y < 0 || n.Y > g.Size {
			t.Errorf("node %+v is outside the %vx%v viewport", n, g.Size, g.Size)
		}
	}
}
