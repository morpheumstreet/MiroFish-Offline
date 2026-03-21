package ontology

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/mirofish-offline/lake/internal/adapters/openai"
)

//go:embed system_prompt.txt
var systemPrompt string

// Service implements ports.OntologyGenerator using an OpenAI-compatible HTTP client.
type Service struct {
	llm *openai.Client
}

func New(llm *openai.Client) *Service {
	return &Service{llm: llm}
}

func (s *Service) Generate(ctx context.Context, documentTexts []string, simulationRequirement string, additionalContext *string) (map[string]any, error) {
	user := buildUserMessage(documentTexts, simulationRequirement, additionalContext)
	raw, err := s.llm.ChatJSON(ctx, systemPrompt, user, 0.3, 4096)
	if err != nil {
		return nil, err
	}
	return validateAndProcess(raw), nil
}

func buildUserMessage(documentTexts []string, simulationRequirement string, additionalContext *string) string {
	combined := strings.Join(documentTexts, "\n\n---\n\n")
	origLen := len(combined)
	if len(combined) > maxTextForLLM {
		combined = combined[:maxTextForLLM]
		combined += fmt.Sprintf("\n\n...(Original text has %d characters, first %d characters extracted for ontology analysis)...", origLen, maxTextForLLM)
	}
	b := strings.Builder{}
	b.WriteString("## Simulation Requirements\n\n")
	b.WriteString(simulationRequirement)
	b.WriteString("\n\n## Document Content\n\n")
	b.WriteString(combined)
	if additionalContext != nil && strings.TrimSpace(*additionalContext) != "" {
		b.WriteString("\n\n## Additional Explanation\n\n")
		b.WriteString(*additionalContext)
	}
	b.WriteString(`
Based on the above content, design entity types and relationship types suitable for social opinion simulation.

**Rules to follow**:
1. Must output exactly 10 entity types
2. Last 2 must be fallback types: Person (individual fallback) and Organization (organization fallback)
3. First 8 are specific types designed based on text content
4. All entity types must be real-world subjects that can voice opinions, not abstract concepts
5. Attribute names cannot use reserved words like name, uuid, group_id, use full_name, org_name, etc. instead
`)
	return b.String()
}
