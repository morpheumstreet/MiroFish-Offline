package neo4j

import (
	"context"
	"fmt"

	"github.com/mirofish-offline/lake/internal/ports"
)

// EntityReader implements ports.EntityReader using graph queries (Python EntityReader parity).
type EntityReader struct {
	gs *GraphStore
}

func NewEntityReader(gs *GraphStore) *EntityReader {
	return &EntityReader{gs: gs}
}

var _ ports.EntityReader = (*EntityReader)(nil)

func (e *EntityReader) FilterDefinedEntities(ctx context.Context, graphID string, entityTypes []string, enrich bool) (map[string]any, error) {
	data, err := e.gs.GetGraphData(ctx, graphID)
	if err != nil {
		return nil, err
	}
	nodeSlice, _ := data["nodes"].([]any)
	var nodes []map[string]any
	for _, x := range nodeSlice {
		if nm, ok := x.(map[string]any); ok {
			nodes = append(nodes, nm)
		}
	}
	edges := []map[string]any{}
	if enrich {
		if ea, ok := data["edges"].([]any); ok {
			for _, x := range ea {
				if em, ok := x.(map[string]any); ok {
					edges = append(edges, em)
				}
			}
		}
	}
	nodeMap := map[string]map[string]any{}
	for _, nm := range nodes {
		u := fmt.Sprint(nm["uuid"])
		nodeMap[u] = nm
	}

	typeFilter := map[string]struct{}{}
	for _, t := range entityTypes {
		typeFilter[t] = struct{}{}
	}

	var filtered []map[string]any
	typeSet := map[string]struct{}{}
	total := len(nodes)

	for _, node := range nodes {
		labelSlice := coerceStringSlice(node["labels"])
		if len(labelSlice) == 0 {
			continue
		}
		custom := make([]string, 0, len(labelSlice))
		for _, s := range labelSlice {
			if s != "Entity" && s != "Node" {
				custom = append(custom, s)
			}
		}
		if len(custom) == 0 {
			continue
		}
		var entityType string
		if len(typeFilter) > 0 {
			var match string
			for _, c := range custom {
				if _, ok := typeFilter[c]; ok {
					match = c
					break
				}
			}
			if match == "" {
				continue
			}
			entityType = match
		} else {
			entityType = custom[0]
		}
		typeSet[entityType] = struct{}{}

		uuid := fmt.Sprint(node["uuid"])
		ent := map[string]any{
			"uuid":          uuid,
			"name":          node["name"],
			"labels":        labelSlice,
			"summary":       node["summary"],
			"attributes":    node["attributes"],
			"related_edges": []map[string]any{},
			"related_nodes": []map[string]any{},
		}

		if enrich {
			relatedEdges, relatedUUIDs := edgesForNode(edges, uuid)
			ent["related_edges"] = relatedEdges
			var relatedNodes []map[string]any
			for ru := range relatedUUIDs {
				if rn, ok := nodeMap[ru]; ok {
					relatedNodes = append(relatedNodes, map[string]any{
						"uuid":    rn["uuid"],
						"name":    rn["name"],
						"labels":  rn["labels"],
						"summary": rn["summary"],
					})
				}
			}
			ent["related_nodes"] = relatedNodes
		}
		filtered = append(filtered, ent)
	}

	et := make([]string, 0, len(typeSet))
	for t := range typeSet {
		et = append(et, t)
	}
	return map[string]any{
		"entities":       filtered,
		"entity_types":   et,
		"total_count":    total,
		"filtered_count": len(filtered),
	}, nil
}

func coerceStringSlice(v any) []string {
	switch x := v.(type) {
	case []string:
		return append([]string(nil), x...)
	case []any:
		out := make([]string, 0, len(x))
		for _, l := range x {
			out = append(out, fmt.Sprint(l))
		}
		return out
	default:
		return nil
	}
}

func edgesForNode(edges []map[string]any, nodeUUID string) ([]map[string]any, map[string]struct{}) {
	related := map[string]struct{}{}
	var out []map[string]any
	for _, edge := range edges {
		src := fmt.Sprint(edge["source_node_uuid"])
		tgt := fmt.Sprint(edge["target_node_uuid"])
		if src == nodeUUID {
			out = append(out, map[string]any{
				"direction":        "outgoing",
				"edge_name":        edge["name"],
				"fact":             edge["fact"],
				"target_node_uuid": tgt,
			})
			related[tgt] = struct{}{}
		} else if tgt == nodeUUID {
			out = append(out, map[string]any{
				"direction":        "incoming",
				"edge_name":        edge["name"],
				"fact":             edge["fact"],
				"source_node_uuid": src,
			})
			related[src] = struct{}{}
		}
	}
	return out, related
}

func (e *EntityReader) GetEntityWithContext(ctx context.Context, graphID, entityUUID string) (map[string]any, error) {
	node, err := e.gs.GetEntityByUUID(ctx, graphID, entityUUID)
	if err != nil {
		return nil, err
	}
	if node == nil || fmt.Sprint(node["uuid"]) == "" {
		return nil, nil
	}
	edges, err := e.gs.ListRelationEdgesForEntity(ctx, graphID, entityUUID)
	if err != nil {
		return nil, err
	}
	relatedEdges, relatedUUIDs := edgesForNode(edges, entityUUID)
	var relatedNodes []map[string]any
	for ru := range relatedUUIDs {
		n2, err := e.gs.GetEntityByUUID(ctx, graphID, ru)
		if err != nil || n2 == nil {
			continue
		}
		relatedNodes = append(relatedNodes, map[string]any{
			"uuid":    n2["uuid"],
			"name":    n2["name"],
			"labels":  n2["labels"],
			"summary": n2["summary"],
		})
	}
	return map[string]any{
		"uuid":          node["uuid"],
		"name":          node["name"],
		"labels":        node["labels"],
		"summary":       node["summary"],
		"attributes":    node["attributes"],
		"related_edges": relatedEdges,
		"related_nodes": relatedNodes,
	}, nil
}

func (e *EntityReader) GetEntitiesByType(ctx context.Context, graphID, entityType string, enrich bool) ([]map[string]any, error) {
	m, err := e.FilterDefinedEntities(ctx, graphID, []string{entityType}, enrich)
	if err != nil {
		return nil, err
	}
	raw, _ := m["entities"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, x := range raw {
		if em, ok := x.(map[string]any); ok {
			out = append(out, em)
		}
	}
	return out, nil
}
