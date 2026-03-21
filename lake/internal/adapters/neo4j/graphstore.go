package neo4j

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	neo4jdriver "github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/mirofish-offline/lake/internal/adapters/ollama"
	"github.com/mirofish-offline/lake/internal/adapters/openai"
	"github.com/mirofish-offline/lake/internal/config"
	"github.com/mirofish-offline/lake/internal/usecase/ner"
)

const neo4jDB = "neo4j"

var safeNeo4jLabel = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// GraphStore implements ports.GraphBuilder and ports.Neo4jHealth (single Bolt driver).
type GraphStore struct {
	driver   neo4jdriver.DriverWithContext
	cfg      config.Config
	llm      *openai.Client
	embedder *ollama.Embedder
}

// NewGraphStore opens the driver, applies schema best-effort, and wires LLM + embeddings.
func NewGraphStore(cfg config.Config, llm *openai.Client) (*GraphStore, error) {
	auth := neo4jdriver.BasicAuth(cfg.Neo4jUser, cfg.Neo4jPassword, "")
	driver, err := neo4jdriver.NewDriverWithContext(cfg.Neo4jURI, auth)
	if err != nil {
		return nil, fmt.Errorf("neo4j driver: %w", err)
	}
	gs := &GraphStore{
		driver:   driver,
		cfg:      cfg,
		llm:      llm,
		embedder: ollama.NewEmbedder(cfg),
	}
	// Do not block HTTP listen on Bolt/schema; same best-effort behavior as before.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		_ = gs.ensureSchema(ctx)
	}()
	return gs, nil
}

func (gs *GraphStore) ensureSchema(ctx context.Context) error {
	sess := gs.driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeWrite, DatabaseName: neo4jDB})
	defer sess.Close(ctx)
	for _, q := range SchemaQueries {
		_, err := sess.ExecuteWrite(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx, q, nil)
			return nil, err
		})
		if err != nil {
			// Same as Python: log and continue (index may exist / edition limits).
			_ = err
		}
	}
	return nil
}

// Ping implements ports.Neo4jHealth.
func (gs *GraphStore) Ping(ctx context.Context) error {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return gs.driver.VerifyConnectivity(cctx)
}

// Close shuts down the driver.
func (gs *GraphStore) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return gs.driver.Close(ctx)
}

// CreateGraph creates a :Graph node (UUID graph_id) like Python Neo4jStorage.create_graph.
func (gs *GraphStore) CreateGraph(ctx context.Context, name string) (string, error) {
	graphID := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	sess := gs.driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeWrite, DatabaseName: neo4jDB})
	defer sess.Close(ctx)
	_, err := sess.ExecuteWrite(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, `
			CREATE (g:Graph {
				graph_id: $graph_id,
				name: $name,
				description: $description,
				ontology_json: '{}',
				created_at: $created_at
			})`,
			map[string]any{
				"graph_id":    graphID,
				"name":        name,
				"description": "MiroFish Social Simulation Graph",
				"created_at":  now,
			},
		)
		return nil, err
	})
	if err != nil {
		return "", err
	}
	return graphID, nil
}

// SetOntology stores ontology JSON on the Graph node.
func (gs *GraphStore) SetOntology(ctx context.Context, graphID string, ontology map[string]any) error {
	b, err := json.Marshal(ontology)
	if err != nil {
		return err
	}
	sess := gs.driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeWrite, DatabaseName: neo4jDB})
	defer sess.Close(ctx)
	_, err = sess.ExecuteWrite(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, `
			MATCH (g:Graph {graph_id: $gid})
			SET g.ontology_json = $ontology_json`,
			map[string]any{"gid": graphID, "ontology_json": string(b)},
		)
		return nil, err
	})
	return err
}

// DeleteGraph removes all nodes tied to graph_id and the Graph node.
func (gs *GraphStore) DeleteGraph(ctx context.Context, graphID string) error {
	sess := gs.driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeWrite, DatabaseName: neo4jDB})
	defer sess.Close(ctx)
	_, err := sess.ExecuteWrite(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, `MATCH (n {graph_id: $gid}) DETACH DELETE n`, map[string]any{"gid": graphID})
		if err != nil {
			return nil, err
		}
		_, err = tx.Run(ctx, `MATCH (g:Graph {graph_id: $gid}) DELETE g`, map[string]any{"gid": graphID})
		return nil, err
	})
	return err
}

func (gs *GraphStore) getOntologyMap(ctx context.Context, graphID string) (map[string]any, error) {
	sess := gs.driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeRead, DatabaseName: neo4jDB})
	defer sess.Close(ctx)
	v, err := sess.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, `MATCH (g:Graph {graph_id: $gid}) RETURN coalesce(g.ontology_json, '') AS oj`, map[string]any{"gid": graphID})
		if err != nil {
			return nil, err
		}
		rec, err := res.Single(ctx)
		if err != nil {
			return map[string]any{}, nil
		}
		oj, _ := rec.Get("oj")
		s, _ := oj.(string)
		if s == "" {
			return map[string]any{}, nil
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(s), &m); err != nil {
			return map[string]any{}, nil
		}
		return m, nil
	})
	if err != nil {
		return nil, err
	}
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}
	return map[string]any{}, nil
}

// addText runs NER, embeddings, and writes Episode + Entity + RELATION (Python Neo4jStorage.add_text).
func (gs *GraphStore) addText(ctx context.Context, graphID, text string) (string, error) {
	episodeID := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	ontology, err := gs.getOntologyMap(ctx, graphID)
	if err != nil {
		return "", err
	}
	ext := ner.Extract(ctx, gs.llm, text, ontology)
	entities, _ := ext["entities"].([]any)
	relations, _ := ext["relations"].([]any)

	entitySummaries := make([]string, 0, len(entities))
	for _, e := range entities {
		em, ok := e.(map[string]any)
		if !ok {
			continue
		}
		entitySummaries = append(entitySummaries, fmt.Sprintf("%s (%s)", em["name"], em["type"]))
	}
	factTexts := make([]string, 0, len(relations))
	for _, r := range relations {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		f := strings.TrimSpace(fmt.Sprint(rm["fact"]))
		if f == "" {
			f = fmt.Sprintf("%s %s %s", rm["source"], rm["type"], rm["target"])
		}
		factTexts = append(factTexts, f)
	}
	allEmbed := append(append([]string{}, entitySummaries...), factTexts...)
	var allVecs [][]float64
	if len(allEmbed) > 0 {
		allVecs, err = gs.embedder.EmbedBatch(ctx, allEmbed)
		if err != nil {
			allVecs = make([][]float64, len(allEmbed))
		}
	}
	ne := len(entitySummaries)
	nf := len(factTexts)
	entityEmb := make([][]float64, ne)
	for i := 0; i < ne; i++ {
		if i < len(allVecs) {
			entityEmb[i] = allVecs[i]
		}
	}
	relEmb := make([][]float64, nf)
	for i := 0; i < nf; i++ {
		j := ne + i
		if j < len(allVecs) {
			relEmb[i] = allVecs[j]
		}
	}

	sess := gs.driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeWrite, DatabaseName: neo4jDB})
	defer sess.Close(ctx)

	_, err = sess.ExecuteWrite(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, `
			CREATE (ep:Episode {
				uuid: $uuid,
				graph_id: $graph_id,
				data: $data,
				processed: true,
				created_at: $created_at
			})`,
			map[string]any{"uuid": episodeID, "graph_id": graphID, "data": text, "created_at": now},
		)
		return nil, err
	})
	if err != nil {
		return "", err
	}

	entityUUIDMap := map[string]string{}

	for idx, e := range entities {
		em, ok := e.(map[string]any)
		if !ok {
			continue
		}
		ename := fmt.Sprint(em["name"])
		etype := fmt.Sprint(em["type"])
		attrs, _ := em["attributes"].(map[string]any)
		if attrs == nil {
			attrs = map[string]any{}
		}
		attrsJSON, _ := json.Marshal(attrs)
		summary := ""
		if idx < len(entitySummaries) {
			summary = entitySummaries[idx]
		}
		emb := []float64{}
		if idx < len(entityEmb) && entityEmb[idx] != nil {
			emb = entityEmb[idx]
		}
		eUUID := uuid.NewString()

		actual, err := gs.mergeEntity(ctx, graphID, eUUID, ename, etype, summary, string(attrsJSON), emb, now)
		if err != nil {
			return "", err
		}
		entityUUIDMap[strings.ToLower(ename)] = actual

		if etype != "" && etype != "Entity" && safeNeo4jLabel.MatchString(etype) {
			_ = gs.addEntityLabel(ctx, graphID, strings.ToLower(ename), etype)
		}
	}

	for idx, r := range relations {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		src := fmt.Sprint(rm["source"])
		tgt := fmt.Sprint(rm["target"])
		rtype := fmt.Sprint(rm["type"])
		fact := strings.TrimSpace(fmt.Sprint(rm["fact"]))
		if fact == "" {
			fact = fmt.Sprintf("%s %s %s", src, rtype, tgt)
		}
		su, ok1 := entityUUIDMap[strings.ToLower(src)]
		tu, ok2 := entityUUIDMap[strings.ToLower(tgt)]
		if !ok1 || !ok2 {
			continue
		}
		fe := []float64{}
		if idx < len(relEmb) && relEmb[idx] != nil {
			fe = relEmb[idx]
		}
		rUUID := uuid.NewString()
		err := gs.createRelation(ctx, su, tu, rUUID, graphID, rtype, fact, fe, episodeID, now)
		if err != nil {
			return "", err
		}
	}

	return episodeID, nil
}

func (gs *GraphStore) mergeEntity(ctx context.Context, graphID, eUUID, name, etype, summary, attrsJSON string, embedding []float64, now string) (string, error) {
	sess := gs.driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeWrite, DatabaseName: neo4jDB})
	defer sess.Close(ctx)
	v, err := sess.ExecuteWrite(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, `
			MERGE (n:Entity {graph_id: $gid, name_lower: $name_lower})
			ON CREATE SET
				n.uuid = $uuid,
				n.name = $name,
				n.summary = $summary,
				n.attributes_json = $attrs_json,
				n.embedding = $embedding,
				n.created_at = $now
			ON MATCH SET
				n.summary = CASE WHEN n.summary = '' OR n.summary IS NULL
					THEN $summary ELSE n.summary END,
				n.attributes_json = $attrs_json,
				n.embedding = $embedding
			RETURN n.uuid AS uuid`,
			map[string]any{
				"gid": graphID, "name_lower": strings.ToLower(name), "uuid": eUUID,
				"name": name, "summary": summary, "attrs_json": attrsJSON,
				"embedding": embedding, "now": now,
			},
		)
		if err != nil {
			return nil, err
		}
		rec, err := res.Single(ctx)
		if err != nil {
			return eUUID, nil
		}
		u, _ := rec.Get("uuid")
		if s, ok := u.(string); ok && s != "" {
			return s, nil
		}
		return eUUID, nil
	})
	if err != nil {
		return "", err
	}
	if s, ok := v.(string); ok {
		return s, nil
	}
	return eUUID, nil
}

func (gs *GraphStore) addEntityLabel(ctx context.Context, graphID, nameLower, label string) error {
	sess := gs.driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeWrite, DatabaseName: neo4jDB})
	defer sess.Close(ctx)
	q := fmt.Sprintf(
		"MATCH (n:Entity {graph_id: $gid, name_lower: $nl}) SET n:`%s`",
		label,
	)
	_, err := sess.ExecuteWrite(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, q, map[string]any{"gid": graphID, "nl": nameLower})
		return nil, err
	})
	return err
}

func (gs *GraphStore) createRelation(ctx context.Context, srcUUID, tgtUUID, rUUID, graphID, name, fact string, factEmb []float64, episodeID, now string) error {
	sess := gs.driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeWrite, DatabaseName: neo4jDB})
	defer sess.Close(ctx)
	_, err := sess.ExecuteWrite(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, `
			MATCH (src:Entity {uuid: $src_uuid})
			MATCH (tgt:Entity {uuid: $tgt_uuid})
			CREATE (src)-[r:RELATION {
				uuid: $uuid,
				graph_id: $gid,
				name: $name,
				fact: $fact,
				fact_embedding: $fact_embedding,
				attributes_json: '{}',
				episode_ids: $episode_ids,
				created_at: $now,
				valid_at: null,
				invalid_at: null,
				expired_at: null
			}]->(tgt)`,
			map[string]any{
				"src_uuid": srcUUID, "tgt_uuid": tgtUUID, "uuid": rUUID, "gid": graphID,
				"name": name, "fact": fact, "fact_embedding": factEmb,
				"episode_ids": []string{episodeID}, "now": now,
			},
		)
		return nil, err
	})
	return err
}

// AddTextBatches implements ports.GraphBuilder (batch progress matches Python graph_builder).
func (gs *GraphStore) AddTextBatches(ctx context.Context, graphID string, chunks []string, batchSize int, onProgress func(msg string, ratio float64)) ([]string, error) {
	if batchSize <= 0 {
		batchSize = 3
	}
	total := len(chunks)
	if total == 0 {
		return nil, nil
	}
	totalBatches := (total + batchSize - 1) / batchSize
	var episodeUUIDs []string
	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}
		batch := chunks[i:end]
		bn := i/batchSize + 1
		if onProgress != nil {
			ratio := float64(i+len(batch)) / float64(total)
			onProgress(fmt.Sprintf("Processing batch %d/%d (%d chunks)...", bn, totalBatches, len(batch)), ratio)
		}
		for _, chunk := range batch {
			if strings.TrimSpace(chunk) == "" {
				continue
			}
			ep, err := gs.addText(ctx, graphID, chunk)
			if err != nil {
				if onProgress != nil {
					onProgress(fmt.Sprintf("Batch %d processing failed: %v", bn, err), 0)
				}
				return nil, err
			}
			episodeUUIDs = append(episodeUUIDs, ep)
		}
	}
	return episodeUUIDs, nil
}

// GetGraphData returns nodes, edges, counts (Python get_graph_data).
func (gs *GraphStore) GetGraphData(ctx context.Context, graphID string) (map[string]any, error) {
	sess := gs.driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeRead, DatabaseName: neo4jDB})
	defer sess.Close(ctx)
	v, err := sess.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, `
			MATCH (n:Entity {graph_id: $gid})
			RETURN n, labels(n) AS labels`,
			map[string]any{"gid": graphID},
		)
		if err != nil {
			return nil, err
		}
		var nodes []map[string]any
		nodeMap := map[string]string{}
		for res.Next(ctx) {
			rec := res.Record()
			node, _ := rec.Get("n")
			labels, _ := rec.Get("labels")
			nd := nodeToDict(node, labels)
			nodes = append(nodes, nd)
			nodeMap[fmt.Sprint(nd["uuid"])] = fmt.Sprint(nd["name"])
		}
		if err := res.Err(); err != nil {
			return nil, err
		}

		res2, err := tx.Run(ctx, `
			MATCH (src:Entity)-[r:RELATION {graph_id: $gid}]->(tgt:Entity)
			RETURN r, src.uuid AS src_uuid, tgt.uuid AS tgt_uuid,
			       src.name AS src_name, tgt.name AS tgt_name`,
			map[string]any{"gid": graphID},
		)
		if err != nil {
			return nil, err
		}
		var edges []map[string]any
		for res2.Next(ctx) {
			rec := res2.Record()
			rel, _ := rec.Get("r")
			su, _ := rec.Get("src_uuid")
			tu, _ := rec.Get("tgt_uuid")
			sn, _ := rec.Get("src_name")
			tn, _ := rec.Get("tgt_name")
			ed := edgeToDict(rel, fmt.Sprint(su), fmt.Sprint(tu))
			ed["fact_type"] = ed["name"]
			ed["source_node_name"] = fmt.Sprint(sn)
			ed["target_node_name"] = fmt.Sprint(tn)
			ed["episodes"] = ed["episode_ids"]
			_ = nodeMap
			edges = append(edges, ed)
		}
		if err := res2.Err(); err != nil {
			return nil, err
		}

		return map[string]any{
			"graph_id":   graphID,
			"nodes":      nodes,
			"edges":      edges,
			"node_count": len(nodes),
			"edge_count": len(edges),
		}, nil
	})
	if err != nil {
		return nil, err
	}
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}
	return nil, fmt.Errorf("unexpected graph data")
}

func nodeToDict(node any, labels any) map[string]any {
	n, ok := node.(neo4jdriver.Node)
	if !ok {
		return map[string]any{}
	}
	props := n.Props
	attrsJSON := "{}"
	if v, ok := props["attributes_json"]; ok {
		attrsJSON = fmt.Sprint(v)
	}
	var attributes map[string]any
	_ = json.Unmarshal([]byte(attrsJSON), &attributes)
	if attributes == nil {
		attributes = map[string]any{}
	}
	name := fmt.Sprint(props["name"])
	uuidStr := fmt.Sprint(props["uuid"])
	lbls := labelStrings(labels)
	var outLbls []string
	for _, l := range lbls {
		if l != "Entity" {
			outLbls = append(outLbls, l)
		}
	}
	return map[string]any{
		"uuid":       uuidStr,
		"name":       name,
		"labels":     outLbls,
		"summary":    fmt.Sprint(props["summary"]),
		"attributes": attributes,
		"created_at": props["created_at"],
	}
}

func labelStrings(labels any) []string {
	switch x := labels.(type) {
	case []string:
		return x
	case []any:
		var o []string
		for _, e := range x {
			o = append(o, fmt.Sprint(e))
		}
		return o
	default:
		return nil
	}
}

func edgeToDict(rel any, srcUUID, tgtUUID string) map[string]any {
	r, ok := rel.(neo4jdriver.Relationship)
	if !ok {
		return map[string]any{}
	}
	props := r.Props
	attrsJSON := "{}"
	if v, ok := props["attributes_json"]; ok {
		attrsJSON = fmt.Sprint(v)
	}
	var attributes map[string]any
	_ = json.Unmarshal([]byte(attrsJSON), &attributes)
	if attributes == nil {
		attributes = map[string]any{}
	}
	epIDs := props["episode_ids"]
	var episodeIDs []any
	switch x := epIDs.(type) {
	case []any:
		episodeIDs = x
	case string:
		episodeIDs = []any{x}
	case nil:
		episodeIDs = nil
	default:
		episodeIDs = []any{fmt.Sprint(x)}
	}
	return map[string]any{
		"uuid":             fmt.Sprint(props["uuid"]),
		"name":             fmt.Sprint(props["name"]),
		"fact":             fmt.Sprint(props["fact"]),
		"source_node_uuid": srcUUID,
		"target_node_uuid": tgtUUID,
		"attributes":       attributes,
		"created_at":       props["created_at"],
		"valid_at":         props["valid_at"],
		"invalid_at":       props["invalid_at"],
		"expired_at":       props["expired_at"],
		"episode_ids":      episodeIDs,
	}
}

// GetEntityByUUID returns one :Entity node scoped to graph_id (for EntityReader).
func (gs *GraphStore) GetEntityByUUID(ctx context.Context, graphID, entityUUID string) (map[string]any, error) {
	sess := gs.driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeRead, DatabaseName: neo4jDB})
	defer sess.Close(ctx)
	v, err := sess.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, `
			MATCH (n:Entity {graph_id: $gid, uuid: $uid})
			RETURN n, labels(n) AS labels`,
			map[string]any{"gid": graphID, "uid": entityUUID},
		)
		if err != nil {
			return nil, err
		}
		if !res.Next(ctx) {
			return nil, nil
		}
		rec := res.Record()
		node, _ := rec.Get("n")
		labels, _ := rec.Get("labels")
		if err := res.Err(); err != nil {
			return nil, err
		}
		return nodeToDict(node, labels), nil
	})
	if err != nil {
		return nil, err
	}
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}
	return nil, nil
}

// ListRelationEdgesForEntity returns RELATION rows touching the entity (same shape as GetGraphData edges).
func (gs *GraphStore) ListRelationEdgesForEntity(ctx context.Context, graphID, entityUUID string) ([]map[string]any, error) {
	sess := gs.driver.NewSession(ctx, neo4jdriver.SessionConfig{AccessMode: neo4jdriver.AccessModeRead, DatabaseName: neo4jDB})
	defer sess.Close(ctx)
	v, err := sess.ExecuteRead(ctx, func(tx neo4jdriver.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, `
			MATCH (src:Entity)-[r:RELATION {graph_id: $gid}]->(tgt:Entity)
			WHERE src.uuid = $uid OR tgt.uuid = $uid
			RETURN r, src.uuid AS src_uuid, tgt.uuid AS tgt_uuid,
			       src.name AS src_name, tgt.name AS tgt_name`,
			map[string]any{"gid": graphID, "uid": entityUUID},
		)
		if err != nil {
			return nil, err
		}
		var edges []map[string]any
		for res.Next(ctx) {
			rec := res.Record()
			rel, _ := rec.Get("r")
			su, _ := rec.Get("src_uuid")
			tu, _ := rec.Get("tgt_uuid")
			sn, _ := rec.Get("src_name")
			tn, _ := rec.Get("tgt_name")
			ed := edgeToDict(rel, fmt.Sprint(su), fmt.Sprint(tu))
			ed["fact_type"] = ed["name"]
			ed["source_node_name"] = fmt.Sprint(sn)
			ed["target_node_name"] = fmt.Sprint(tn)
			ed["episodes"] = ed["episode_ids"]
			edges = append(edges, ed)
		}
		return edges, res.Err()
	})
	if err != nil {
		return nil, err
	}
	if edges, ok := v.([]map[string]any); ok {
		return edges, nil
	}
	return nil, nil
}
