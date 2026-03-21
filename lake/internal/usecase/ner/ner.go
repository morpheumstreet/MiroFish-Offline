package ner

import (
	"context"
	"fmt"
	"strings"

	"github.com/mirofish-offline/lake/internal/adapters/openai"
)

const nerSystemTemplate = `You are a Named Entity Recognition and Relation Extraction system.
Given a text and an ontology (entity types + relation types), extract all entities and relations.

ONTOLOGY:
%s

RULES:
1. Only extract entity types and relation types defined in the ontology.
2. Normalize entity names: strip whitespace, use canonical form (e.g., "Jack Ma" not "ma jack").
3. Each entity must have: name, type (from ontology), and optional attributes.
4. Each relation must have: source entity name, target entity name, type (from ontology), and a fact sentence describing the relationship.
5. If no entities or relations are found, return empty lists.
6. Be precise — only extract what is explicitly stated or strongly implied in the text.

Return ONLY valid JSON in this exact format:
{
  "entities": [
    {"name": "...", "type": "...", "attributes": {"key": "value"}}
  ],
  "relations": [
    {"source": "...", "target": "...", "type": "...", "fact": "..."}
  ]
}`

const nerUserTemplate = `Extract entities and relations from the following text:

%s`

// Extract calls the LLM and normalizes output (Python NERExtractor.extract + _validate_and_clean).
func Extract(ctx context.Context, client *openai.Client, text string, ontology map[string]any) map[string]any {
	if strings.TrimSpace(text) == "" {
		return map[string]any{"entities": []any{}, "relations": []any{}}
	}
	desc := formatOntology(ontology)
	system := fmt.Sprintf(nerSystemTemplate, desc)
	user := fmt.Sprintf(nerUserTemplate, strings.TrimSpace(text))
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		raw, err := client.ChatJSONMessages(ctx, []openai.ChatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		}, 0.1, 4096)
		if err != nil {
			lastErr = err
			continue
		}
		return validateAndClean(raw, ontology)
	}
	_ = lastErr
	return map[string]any{"entities": []any{}, "relations": []any{}}
}

func formatOntology(ontology map[string]any) string {
	var parts []string
	if et, ok := ontology["entity_types"].([]any); ok && len(et) > 0 {
		parts = append(parts, "Entity Types:")
		for _, x := range et {
			m, ok := x.(map[string]any)
			if !ok {
				parts = append(parts, fmt.Sprintf("  - %v", x))
				continue
			}
			name := fmt.Sprint(m["name"])
			desc := fmt.Sprint(m["description"])
			line := "  - " + name
			if desc != "" {
				line += ": " + desc
			}
			if attrs, ok := m["attributes"].([]any); ok && len(attrs) > 0 {
				var names []string
				for _, a := range attrs {
					if am, ok := a.(map[string]any); ok {
						names = append(names, fmt.Sprint(am["name"]))
					} else {
						names = append(names, fmt.Sprint(a))
					}
				}
				line += fmt.Sprintf(" (attributes: %s)", strings.Join(names, ", "))
			}
			parts = append(parts, line)
		}
	}
	rels, _ := ontology["edge_types"].([]any)
	if len(rels) == 0 {
		rels, _ = ontology["relation_types"].([]any)
	}
	if len(rels) > 0 {
		parts = append(parts, "\nRelation Types:")
		for _, x := range rels {
			m, ok := x.(map[string]any)
			if !ok {
				parts = append(parts, fmt.Sprintf("  - %v", x))
				continue
			}
			name := fmt.Sprint(m["name"])
			desc := fmt.Sprint(m["description"])
			line := "  - " + name
			if desc != "" {
				line += ": " + desc
			}
			if st, ok := m["source_targets"].([]any); ok && len(st) > 0 {
				var stStrs []string
				for _, s := range st {
					if sm, ok := s.(map[string]any); ok {
						stStrs = append(stStrs, fmt.Sprintf("%s → %s", sm["source"], sm["target"]))
					}
				}
				if len(stStrs) > 0 {
					line += fmt.Sprintf(" (%s)", strings.Join(stStrs, ", "))
				}
			}
			parts = append(parts, line)
		}
	}
	if len(parts) == 0 {
		return "No specific ontology defined. Extract all entities and relations you find."
	}
	return strings.Join(parts, "\n")
}

func validateAndClean(result map[string]any, ontology map[string]any) map[string]any {
	entities, _ := result["entities"].([]any)
	relations, _ := result["relations"].([]any)

	validET := map[string]struct{}{}
	for _, et := range sliceOfMaps(ontology["entity_types"]) {
		validET[strings.TrimSpace(fmt.Sprint(et["name"]))] = struct{}{}
	}
	validRT := map[string]struct{}{}
	for _, rt := range sliceOfMaps(ontology["edge_types"]) {
		validRT[strings.TrimSpace(fmt.Sprint(rt["name"]))] = struct{}{}
	}
	for _, rt := range sliceOfMaps(ontology["relation_types"]) {
		validRT[strings.TrimSpace(fmt.Sprint(rt["name"]))] = struct{}{}
	}

	var cleanedEnts []map[string]any
	seen := map[string]struct{}{}
	for _, e := range entities {
		em, ok := e.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprint(em["name"]))
		etype := strings.TrimSpace(fmt.Sprint(em["type"]))
		if etype == "" {
			etype = "Entity"
		}
		if name == "" {
			continue
		}
		nl := strings.ToLower(name)
		if _, dup := seen[nl]; dup {
			continue
		}
		seen[nl] = struct{}{}
		_ = validET
		attrs, _ := em["attributes"].(map[string]any)
		if attrs == nil {
			attrs = map[string]any{}
		}
		cleanedEnts = append(cleanedEnts, map[string]any{
			"name":       name,
			"type":       etype,
			"attributes": attrs,
		})
	}

	entityNamesLower := map[string]struct{}{}
	for _, e := range cleanedEnts {
		entityNamesLower[strings.ToLower(fmt.Sprint(e["name"]))] = struct{}{}
	}

	var cleanedRels []map[string]any
	for _, r := range relations {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		src := strings.TrimSpace(fmt.Sprint(rm["source"]))
		tgt := strings.TrimSpace(fmt.Sprint(rm["target"]))
		rtype := strings.TrimSpace(fmt.Sprint(rm["type"]))
		if rtype == "" {
			rtype = "RELATED_TO"
		}
		fact := strings.TrimSpace(fmt.Sprint(rm["fact"]))
		if src == "" || tgt == "" {
			continue
		}
		if _, ok := entityNamesLower[strings.ToLower(src)]; !ok {
			cleanedEnts = append(cleanedEnts, map[string]any{"name": src, "type": "Entity", "attributes": map[string]any{}})
			entityNamesLower[strings.ToLower(src)] = struct{}{}
		}
		if _, ok := entityNamesLower[strings.ToLower(tgt)]; !ok {
			cleanedEnts = append(cleanedEnts, map[string]any{"name": tgt, "type": "Entity", "attributes": map[string]any{}})
			entityNamesLower[strings.ToLower(tgt)] = struct{}{}
		}
		if fact == "" {
			fact = fmt.Sprintf("%s %s %s", src, rtype, tgt)
		}
		_ = validRT
		cleanedRels = append(cleanedRels, map[string]any{
			"source": src, "target": tgt, "type": rtype, "fact": fact,
		})
	}

	outE := make([]any, len(cleanedEnts))
	for i, e := range cleanedEnts {
		outE[i] = e
	}
	outR := make([]any, len(cleanedRels))
	for i, r := range cleanedRels {
		outR[i] = r
	}
	return map[string]any{"entities": outE, "relations": outR}
}

func sliceOfMaps(v any) []map[string]any {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	var out []map[string]any
	for _, x := range arr {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
