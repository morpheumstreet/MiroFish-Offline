package ontology

import (
	"fmt"
	"strings"
)

const maxEntityTypes = 10
const maxEdgeTypes = 10
const maxTextForLLM = 50000

func validateAndProcess(result map[string]any) map[string]any {
	if result == nil {
		result = map[string]any{}
	}
	if _, ok := result["entity_types"]; !ok {
		result["entity_types"] = []any{}
	}
	if _, ok := result["edge_types"]; !ok {
		result["edge_types"] = []any{}
	}
	if _, ok := result["analysis_summary"]; !ok {
		result["analysis_summary"] = ""
	}

	entityRaw, _ := result["entity_types"].([]any)
	cleanedEntities := make([]any, 0, len(entityRaw))
	for _, item := range entityRaw {
		ent, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(ent["name"]))
		if name == "" {
			for _, alt := range []string{"type", "label", "entity_type", "entity_name", "title"} {
				if v, ok := ent[alt]; ok {
					s := strings.TrimSpace(fmt.Sprint(v))
					if s != "" {
						ent["name"] = s
						name = s
						break
					}
				}
			}
		}
		if strings.TrimSpace(fmt.Sprint(ent["name"])) == "" {
			ent["name"] = fmt.Sprintf("EntityType_%d", len(cleanedEntities)+1)
		}
		if _, ok := ent["attributes"]; !ok {
			ent["attributes"] = []any{}
		}
		if _, ok := ent["examples"]; !ok {
			ent["examples"] = []any{}
		}
		if desc, ok := ent["description"].(string); ok && len(desc) > 100 {
			ent["description"] = desc[:97] + "..."
		}
		cleanedEntities = append(cleanedEntities, ent)
	}
	result["entity_types"] = cleanedEntities

	edgeRaw, _ := result["edge_types"].([]any)
	cleanedEdges := make([]any, 0, len(edgeRaw))
	for _, item := range edgeRaw {
		edge, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(edge["name"]))
		if name == "" {
			for _, alt := range []string{"type", "label", "relation_type", "edge_type"} {
				if v, ok := edge[alt]; ok {
					s := strings.TrimSpace(fmt.Sprint(v))
					if s != "" {
						edge["name"] = s
						name = s
						break
					}
				}
			}
		}
		if strings.TrimSpace(fmt.Sprint(edge["name"])) == "" {
			edge["name"] = fmt.Sprintf("RELATION_%d", len(cleanedEdges)+1)
		}
		if _, ok := edge["source_targets"]; !ok {
			edge["source_targets"] = []any{}
		}
		if _, ok := edge["attributes"]; !ok {
			edge["attributes"] = []any{}
		}
		if desc, ok := edge["description"].(string); ok && len(desc) > 100 {
			edge["description"] = desc[:97] + "..."
		}
		cleanedEdges = append(cleanedEdges, edge)
	}
	result["edge_types"] = cleanedEdges

	personFallback := map[string]any{
		"name":        "Person",
		"description": "Any individual person not fitting other specific person types.",
		"attributes": []any{
			map[string]any{"name": "full_name", "type": "text", "description": "Full name of the person"},
			map[string]any{"name": "role", "type": "text", "description": "Role or occupation"},
		},
		"examples": []any{"ordinary citizen", "anonymous netizen"},
	}
	orgFallback := map[string]any{
		"name":        "Organization",
		"description": "Any organization not fitting other specific organization types.",
		"attributes": []any{
			map[string]any{"name": "org_name", "type": "text", "description": "Name of the organization"},
			map[string]any{"name": "org_type", "type": "text", "description": "Type of organization"},
		},
		"examples": []any{"small business", "community group"},
	}

	entities, _ := result["entity_types"].([]any)
	entityNames := map[string]struct{}{}
	for _, e := range entities {
		if m, ok := e.(map[string]any); ok {
			entityNames[strings.TrimSpace(fmt.Sprint(m["name"]))] = struct{}{}
		}
	}
	_, hasPerson := entityNames["Person"]
	_, hasOrg := entityNames["Organization"]

	var fallbacks []map[string]any
	if !hasPerson {
		fallbacks = append(fallbacks, personFallback)
	}
	if !hasOrg {
		fallbacks = append(fallbacks, orgFallback)
	}
	if len(fallbacks) > 0 {
		current := len(entities)
		needed := len(fallbacks)
		if current+needed > maxEntityTypes {
			toRemove := current + needed - maxEntityTypes
			if toRemove > 0 && toRemove <= len(entities) {
				entities = entities[:len(entities)-toRemove]
			}
		}
		for _, fb := range fallbacks {
			entities = append(entities, fb)
		}
		result["entity_types"] = entities
	}

	entities, _ = result["entity_types"].([]any)
	if len(entities) > maxEntityTypes {
		result["entity_types"] = entities[:maxEntityTypes]
	}
	edges, _ := result["edge_types"].([]any)
	if len(edges) > maxEdgeTypes {
		result["edge_types"] = edges[:maxEdgeTypes]
	}

	return result
}
