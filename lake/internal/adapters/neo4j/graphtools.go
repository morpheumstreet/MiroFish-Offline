package neo4j

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mirofish-offline/lake/internal/ports"
)

// GraphTools implements ports.GraphTools using keyword scoring over GetGraphData (pragmatic InsightForge-lite).
type GraphTools struct {
	gs *GraphStore
}

var _ ports.GraphTools = (*GraphTools)(nil)

// NewGraphTools builds tools backed by an existing GraphStore.
func NewGraphTools(gs *GraphStore) *GraphTools {
	return &GraphTools{gs: gs}
}

func tokenize(q string) []string {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	var out []string
	for _, w := range strings.Fields(q) {
		w = strings.Trim(w, ".,;:!?\"'()[]")
		if len(w) > 1 {
			out = append(out, w)
		}
	}
	if len(out) == 0 {
		out = append(out, q)
	}
	return out
}

func scoreText(text string, terms []string) int {
	text = strings.ToLower(text)
	s := 0
	for _, t := range terms {
		if t != "" && strings.Contains(text, t) {
			s += 2
		}
	}
	return s
}

type scoredNode struct {
	n  map[string]any
	sc int
}

// SearchGraph returns nodes/edges/facts compatible with Python SearchResult.to_dict.
func (t *GraphTools) SearchGraph(ctx context.Context, graphID, query string, limit int) (map[string]any, error) {
	if limit <= 0 {
		limit = 15
	}
	data, err := t.gs.GetGraphData(ctx, graphID)
	if err != nil {
		return nil, err
	}
	nodes, _ := data["nodes"].([]any)
	edges, _ := data["edges"].([]any)
	terms := tokenize(query)

	var ranked []scoredNode
	for _, x := range nodes {
		n, ok := x.(map[string]any)
		if !ok {
			continue
		}
		name := fmt.Sprint(n["name"])
		summary := fmt.Sprint(n["summary"])
		attrJSON, _ := json.Marshal(n["attributes"])
		sc := scoreText(name, terms)*4 + scoreText(summary, terms)*2 + scoreText(string(attrJSON), terms)
		if sc == 0 && len(terms) > 0 {
			continue
		}
		ranked = append(ranked, scoredNode{n: n, sc: sc})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].sc != ranked[j].sc {
			return ranked[i].sc > ranked[j].sc
		}
		return fmt.Sprint(ranked[i].n["name"]) < fmt.Sprint(ranked[j].n["name"])
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	picked := map[string]struct{}{}
	var outNodes []any
	for _, r := range ranked {
		u := fmt.Sprint(r.n["uuid"])
		picked[u] = struct{}{}
		outNodes = append(outNodes, r.n)
	}

	var outEdges []any
	var facts []string
	seenEdge := map[string]struct{}{}
	for _, x := range edges {
		e, ok := x.(map[string]any)
		if !ok {
			continue
		}
		su := fmt.Sprint(e["source_uuid"])
		tu := fmt.Sprint(e["target_uuid"])
		_, okS := picked[su]
		_, okT := picked[tu]
		if !okS && !okT {
			continue
		}
		ename := fmt.Sprint(e["name"])
		sn := fmt.Sprint(e["source_node_name"])
		tn := fmt.Sprint(e["target_node_name"])
		line := fmt.Sprintf("%s --[%s]--> %s", sn, ename, tn)
		u := fmt.Sprint(e["uuid"])
		if _, dup := seenEdge[u]; dup {
			continue
		}
		seenEdge[u] = struct{}{}
		outEdges = append(outEdges, e)
		if len(facts) < limit*2 {
			facts = append(facts, line)
		}
	}

	// If query matched nothing, return top nodes by degree proxy (first N) so the LLM still gets context.
	if len(outNodes) == 0 && len(nodes) > 0 {
		for i := 0; i < len(nodes) && i < limit; i++ {
			if n, ok := nodes[i].(map[string]any); ok {
				outNodes = append(outNodes, n)
			}
		}
	}

	return map[string]any{
		"facts":       facts,
		"edges":       outEdges,
		"nodes":       outNodes,
		"query":       query,
		"total_count": len(outNodes) + len(outEdges),
	}, nil
}

// GetGraphStatistics returns coarse counts and label histograms for the graph_id.
func (t *GraphTools) GetGraphStatistics(ctx context.Context, graphID string) (map[string]any, error) {
	data, err := t.gs.GetGraphData(ctx, graphID)
	if err != nil {
		return nil, err
	}
	nodes, _ := data["nodes"].([]any)
	edges, _ := data["edges"].([]any)
	labelCounts := map[string]int{}
	for _, x := range nodes {
		n, ok := x.(map[string]any)
		if !ok {
			continue
		}
		switch labels := n["labels"].(type) {
		case []any:
			for _, lb := range labels {
				labelCounts[fmt.Sprint(lb)]++
			}
		case []string:
			for _, lb := range labels {
				labelCounts[lb]++
			}
		}
	}
	return map[string]any{
		"graph_id":     graphID,
		"node_count":   len(nodes),
		"edge_count":   len(edges),
		"entity_types": labelCounts,
	}, nil
}
