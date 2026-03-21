package neo4j

// SchemaQueries mirror backend/app/storage/neo4j_schema.py (IF NOT EXISTS).
var SchemaQueries = []string{
	`CREATE CONSTRAINT graph_uuid IF NOT EXISTS FOR (g:Graph) REQUIRE g.graph_id IS UNIQUE`,
	`CREATE CONSTRAINT entity_uuid IF NOT EXISTS FOR (n:Entity) REQUIRE n.uuid IS UNIQUE`,
	`CREATE CONSTRAINT episode_uuid IF NOT EXISTS FOR (ep:Episode) REQUIRE ep.uuid IS UNIQUE`,
	"CREATE VECTOR INDEX entity_embedding IF NOT EXISTS\n" +
		"FOR (n:Entity) ON (n.embedding)\n" +
		"OPTIONS {indexConfig: {\n" +
		"    `vector.dimensions`: 768,\n" +
		"    `vector.similarity_function`: 'cosine'\n" +
		"}}",
	"CREATE VECTOR INDEX fact_embedding IF NOT EXISTS\n" +
		"FOR ()-[r:RELATION]-() ON (r.fact_embedding)\n" +
		"OPTIONS {indexConfig: {\n" +
		"    `vector.dimensions`: 768,\n" +
		"    `vector.similarity_function`: 'cosine'\n" +
		"}}",
	`CREATE FULLTEXT INDEX entity_fulltext IF NOT EXISTS FOR (n:Entity) ON EACH [n.name, n.summary]`,
	`CREATE FULLTEXT INDEX fact_fulltext IF NOT EXISTS FOR ()-[r:RELATION]-() ON EACH [r.fact, r.name]`,
}
