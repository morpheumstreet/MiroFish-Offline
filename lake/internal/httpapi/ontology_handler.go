package httpapi

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

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

func (s *Server) handleOntologyGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	ctx := r.Context()
	maxBytes := s.deps.Config.MaxUploadBytes
	if maxBytes <= 0 {
		maxBytes = 50 << 20
	}
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		fail(w, http.StatusBadRequest, "invalid multipart form: "+err.Error())
		return
	}
	simulationRequirement := r.FormValue("simulation_requirement")
	projectName := r.FormValue("project_name")
	if projectName == "" {
		projectName = "Unnamed Project"
	}
	additional := r.FormValue("additional_context")
	var additionalPtr *string
	if strings.TrimSpace(additional) != "" {
		additionalPtr = &additional
	}

	if strings.TrimSpace(simulationRequirement) == "" {
		fail(w, http.StatusBadRequest, "Please provide simulation requirement description (simulation_requirement)")
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		fail(w, http.StatusBadRequest, "Please upload at least one document file")
		return
	}

	proj, err := s.deps.Projects.CreateProject(ctx, projectName)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
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
			fail(w, http.StatusBadRequest, "open upload: "+err.Error())
			return
		}
		info, err := s.deps.Projects.SaveUploadedFile(ctx, pid, fh.Filename, f, fh.Size)
		_ = f.Close()
		if err != nil {
			_, _ = s.deps.Projects.DeleteProject(ctx, pid)
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
		raw, err := s.deps.Files.ExtractText(info.Path)
		if err != nil {
			_, _ = s.deps.Projects.DeleteProject(ctx, pid)
			fail(w, http.StatusInternalServerError, "extract text: "+err.Error())
			return
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
		fail(w, http.StatusBadRequest, "No documents successfully processed. Please check file format")
		return
	}

	proj.TotalTextLength = len(allText.String())
	proj.SimulationRequirement = simulationRequirement
	if err := s.deps.Projects.SaveExtractedText(ctx, pid, allText.String()); err != nil {
		_, _ = s.deps.Projects.DeleteProject(ctx, pid)
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	ontology, err := s.deps.Ontology.Generate(ctx, documentTexts, simulationRequirement, additionalPtr)
	if err != nil {
		_, _ = s.deps.Projects.DeleteProject(ctx, pid)
		fail(w, http.StatusInternalServerError, err.Error())
		return
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
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
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
