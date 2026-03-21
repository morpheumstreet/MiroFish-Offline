package httpapi

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/mirofish-offline/lake/internal/domain"
)

func allowedUploadExt(filename string) bool {
	if filename == "" || !strings.Contains(filename, ".") {
		return false
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	switch ext {
	case "pdf", "md", "txt", "markdown":
		return true
	default:
		return false
	}
}

func (s *Server) handleOntologyGenerate(c *fiber.Ctx) error {
	ctx := s.reqCtx(c)
	maxBytes := s.deps.Config.MaxUploadBytes
	if maxBytes <= 0 {
		maxBytes = 50 << 20
	}
	form, err := c.MultipartForm()
	if err != nil {
		return failResp(c, fiber.StatusBadRequest, "invalid multipart form: "+err.Error())
	}
	defer func() { _ = form.RemoveAll() }()

	simulationRequirement := ""
	if v := form.Value["simulation_requirement"]; len(v) > 0 {
		simulationRequirement = v[0]
	}
	projectName := "Unnamed Project"
	if v := form.Value["project_name"]; len(v) > 0 && strings.TrimSpace(v[0]) != "" {
		projectName = v[0]
	}
	additional := ""
	if v := form.Value["additional_context"]; len(v) > 0 {
		additional = v[0]
	}
	var additionalPtr *string
	if strings.TrimSpace(additional) != "" {
		additionalPtr = &additional
	}

	if strings.TrimSpace(simulationRequirement) == "" {
		return failResp(c, fiber.StatusBadRequest, "Please provide simulation requirement description (simulation_requirement)")
	}

	files := form.File["files"]
	if len(files) == 0 {
		return failResp(c, fiber.StatusBadRequest, "Please upload at least one document file")
	}

	proj, err := s.deps.Projects.CreateProject(ctx, projectName)
	if err != nil {
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}
	pid := proj.ProjectID

	var documentTexts []string
	var allText strings.Builder
	proj.Files = nil

	for _, fh := range files {
		if fh == nil || fh.Filename == "" {
			continue
		}
		if !allowedUploadExt(fh.Filename) {
			continue
		}
		f, err := fh.Open()
		if err != nil {
			_, _ = s.deps.Projects.DeleteProject(ctx, pid)
			return failResp(c, fiber.StatusBadRequest, "open upload: "+err.Error())
		}
		info, err := s.deps.Projects.SaveUploadedFile(ctx, pid, fh.Filename, f, fh.Size)
		_ = f.Close()
		if err != nil {
			_, _ = s.deps.Projects.DeleteProject(ctx, pid)
			return failResp(c, fiber.StatusInternalServerError, err.Error())
		}
		raw, err := s.deps.Files.ExtractText(info.Path)
		if err != nil {
			_, _ = s.deps.Projects.DeleteProject(ctx, pid)
			return failResp(c, fiber.StatusInternalServerError, "extract text: "+err.Error())
		}
		text := s.deps.Text.Preprocess(raw)
		documentTexts = append(documentTexts, text)
		proj.Files = append(proj.Files, domain.FileRef{
			Filename: info.OriginalFilename,
			Size:     info.Size,
		})
		fmt.Fprintf(&allText, "\n\n=== %s ===\n%s", info.OriginalFilename, text)
	}

	if len(documentTexts) == 0 {
		_, _ = s.deps.Projects.DeleteProject(ctx, pid)
		return failResp(c, fiber.StatusBadRequest, "No documents successfully processed. Please check file format")
	}

	proj.TotalTextLength = len(allText.String())
	proj.SimulationRequirement = simulationRequirement
	if err := s.deps.Projects.SaveExtractedText(ctx, pid, allText.String()); err != nil {
		_, _ = s.deps.Projects.DeleteProject(ctx, pid)
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}

	ontology, err := s.deps.Ontology.Generate(ctx, documentTexts, simulationRequirement, additionalPtr)
	if err != nil {
		_, _ = s.deps.Projects.DeleteProject(ctx, pid)
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}

	entityTypes, _ := ontology["entity_types"].([]any)
	edgeTypes, _ := ontology["edge_types"].([]any)
	analysisSummary := ""
	if v, ok := ontology["analysis_summary"].(string); ok {
		analysisSummary = v
	} else if ontology["analysis_summary"] != nil {
		analysisSummary = strings.TrimSpace(fmt.Sprint(ontology["analysis_summary"]))
	}

	proj.Ontology = map[string]any{
		"entity_types": entityTypes,
		"edge_types":   edgeTypes,
	}
	proj.AnalysisSummary = analysisSummary
	proj.Status = domain.StatusOntologyGenerated
	if err := s.deps.Projects.SaveProject(ctx, proj); err != nil {
		_, _ = s.deps.Projects.DeleteProject(ctx, pid)
		return failResp(c, fiber.StatusInternalServerError, err.Error())
	}

	return sendJSON(c, fiber.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"project_id":        proj.ProjectID,
			"project_name":      proj.Name,
			"ontology":          proj.Ontology,
			"analysis_summary":  proj.AnalysisSummary,
			"files":             proj.Files,
			"total_text_length": proj.TotalTextLength,
		},
	})
}
