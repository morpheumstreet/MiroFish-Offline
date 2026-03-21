package report

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mirofish-offline/lake/internal/adapters/openai"
	"github.com/mirofish-offline/lake/internal/domain"
	"github.com/mirofish-offline/lake/internal/ports"
)

type runParams struct {
	simulationID, graphID, reportID, taskID, requirement string
}

func intPtr(n int) *int       { return &n }
func strPtr(s string) *string { return &s }

func (s *Service) runGenerate(ctx context.Context, p runParams) {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			s.failRun(p, fmt.Errorf("panic: %v", r))
		}
	}()

	log := func(action, stage string, details map[string]any, sectionTitle *string, sectionIndex *int) {
		elapsed := math.Round(time.Since(start).Seconds()*100) / 100
		m := map[string]any{
			"timestamp":       time.Now().Format(time.RFC3339Nano),
			"elapsed_seconds": elapsed,
			"report_id":       p.reportID,
			"action":          action,
			"stage":           stage,
			"details":         details,
		}
		if sectionTitle != nil {
			m["section_title"] = *sectionTitle
		}
		if sectionIndex != nil {
			m["section_index"] = *sectionIndex
		}
		_ = s.Repo.AppendAgentLog(p.reportID, m)
	}

	now := time.Now().Format(time.RFC3339Nano)
	if err := s.Repo.EnsureFolder(p.reportID); err != nil {
		s.failRun(p, err)
		return
	}

	meta0 := map[string]any{
		"report_id":              p.reportID,
		"simulation_id":          p.simulationID,
		"graph_id":               p.graphID,
		"simulation_requirement": p.requirement,
		"status":                 "planning",
		"outline":                nil,
		"markdown_content":       "",
		"created_at":             now,
		"completed_at":           "",
		"error":                  nil,
	}
	if err := s.Repo.SaveMeta(p.reportID, meta0); err != nil {
		s.failRun(p, err)
		return
	}
	_ = s.Repo.SaveProgress(p.reportID, map[string]any{
		"status":             "planning",
		"progress":           5,
		"message":            "Planning outline",
		"current_section":    nil,
		"completed_sections": []any{},
		"updated_at":         now,
	})

	log("report_start", "pending", map[string]any{
		"simulation_id":          p.simulationID,
		"graph_id":               p.graphID,
		"simulation_requirement": p.requirement,
		"message":                "Report generation task started",
	}, nil, nil)
	s.console(p.reportID, "Report generation started")

	stProc := domain.TaskProcessing
	_ = s.Tasks.UpdateTask(ctx, p.taskID, ports.TaskPatch{
		Status:   &stProc,
		Progress: intPtr(0),
		Message:  strPtr("[planning] Initializing Report Agent..."),
	})

	log("planning_start", "planning", map[string]any{"message": "Started planning report outline"}, nil, nil)

	stats, err := s.Tools.GetGraphStatistics(ctx, p.graphID)
	if err != nil {
		s.failRun(p, fmt.Errorf("graph statistics: %w", err))
		return
	}
	search, err := s.Tools.SearchGraph(ctx, p.graphID, p.requirement, 20)
	if err != nil {
		s.failRun(p, fmt.Errorf("graph search: %w", err))
		return
	}
	compactCtx := compactPlanningContext(stats, search)
	log("planning_context", "planning", map[string]any{
		"message": "Acquired simulation context information",
		"context": compactCtx,
	}, nil, nil)

	statsJSON, _ := json.Marshal(stats)
	retrieval := truncate(searchToText(search), 12000)
	system := `You are a senior analyst planning a structured report. Respond with ONLY valid JSON (no markdown fences) with keys:
title (string), summary (string), sections (array of objects with key "title" string).
Use 4–8 sections with clear analytical titles.`
	user := fmt.Sprintf("Simulation requirement:\n%s\n\nGraph statistics (JSON):\n%s\n\nRetrieved facts and entities:\n%s",
		p.requirement, string(statsJSON), retrieval)
	outlineRaw, err := s.LLM.ChatJSONMessages(ctx, []openai.ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}, 0.35, 2048)
	if err != nil {
		s.failRun(p, fmt.Errorf("outline llm: %w", err))
		return
	}
	outlineMap := normalizeOutlineMap(outlineRaw, p.requirement)
	if err := s.Repo.SaveOutline(p.reportID, outlineMap); err != nil {
		s.failRun(p, err)
		return
	}
	log("planning_complete", "planning", map[string]any{
		"message": "Outline planning completed",
		"outline": outlineMap,
	}, nil, nil)
	s.console(p.reportID, "Outline ready")

	_ = s.Repo.SaveMeta(p.reportID, mergeMeta(meta0, map[string]any{
		"status":  "generating",
		"outline": outlineMap,
	}))
	_ = s.Repo.SaveProgress(p.reportID, map[string]any{
		"status":             "generating",
		"progress":           25,
		"message":            "Generating sections",
		"current_section":    nil,
		"completed_sections": []any{},
		"updated_at":         time.Now().Format(time.RFC3339Nano),
	})
	s.patchTask(p.taskID, 25, "[generating] Outline ready")

	sections, _ := outlineMap["sections"].([]any)
	total := len(sections)
	if total == 0 {
		s.failRun(p, fmt.Errorf("empty outline"))
		return
	}

	var completed []any
	var priorBodies []string

	for i := range sections {
		sm, ok := sections[i].(map[string]any)
		if !ok {
			continue
		}
		title := strings.TrimSpace(asString(sm["title"]))
		if title == "" {
			title = fmt.Sprintf("Section %d", i+1)
		}
		idx := i + 1
		log("section_start", "generating", map[string]any{"message": fmt.Sprintf("Started generating section: %s", title)}, &title, &idx)

		q := p.requirement + " " + title
		secSearch, _ := s.Tools.SearchGraph(ctx, p.graphID, q, 15)
		retrieval := truncate(searchToText(secSearch), 10000)
		prev := strings.Join(priorBodies, "\n---\n")
		if len(prev) > 6000 {
			prev = prev[:6000] + "\n... [truncated]"
		}
		sys := `You are writing ONE section of an analytical simulation report. Output markdown body only (no # title line; subheadings ## or ### allowed). Ground claims in the retrieval text; name key entities when relevant.`
		user := fmt.Sprintf("Report: %s\nExecutive summary: %s\nPreviously written sections (excerpt):\n%s\n\nRetrieval for this section:\n%s\n\nWrite the section: %s",
			asString(outlineMap["title"]), asString(outlineMap["summary"]), prev, retrieval, title)
		body, err := s.LLM.ChatText(ctx, []openai.ChatMessage{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		}, 0.4, 4096)
		if err != nil {
			s.failRun(p, fmt.Errorf("section %d: %w", idx, err))
			return
		}
		if err := s.Repo.SaveSection(p.reportID, idx, title, body); err != nil {
			s.failRun(p, err)
			return
		}
		full := "## " + title + "\n\n" + strings.TrimSpace(body) + "\n\n"
		log("section_complete", "generating", map[string]any{
			"content":        full,
			"content_length": len(full),
			"message":        fmt.Sprintf("Section %s generation completed", title),
		}, &title, &idx)
		priorBodies = append(priorBodies, truncate(body, 800))
		completed = append(completed, title)

		pct := 25 + (idx * 70 / total)
		if pct > 95 {
			pct = 95
		}
		msg := fmt.Sprintf("[generating] Section %d/%d: %s", idx, total, title)
		s.patchTask(p.taskID, pct, msg)
		_ = s.Repo.SaveProgress(p.reportID, map[string]any{
			"status":             "generating",
			"progress":           pct,
			"message":            msg,
			"current_section":    title,
			"completed_sections": completed,
			"updated_at":         time.Now().Format(time.RFC3339Nano),
		})
		s.console(p.reportID, fmt.Sprintf("Section %d/%d done: %s", idx, total, title))
	}

	fullMD, err := s.Repo.AssembleFullMarkdown(p.reportID, outlineMap)
	if err != nil {
		s.failRun(p, err)
		return
	}
	if err := s.Repo.SaveFullMarkdown(p.reportID, fullMD); err != nil {
		s.failRun(p, err)
		return
	}

	filledOutline := outlineWithContents(s.Repo, p.reportID, outlineMap)
	doneAt := time.Now().Format(time.RFC3339Nano)
	finalMeta := mergeMeta(meta0, map[string]any{
		"status":           "completed",
		"outline":          filledOutline,
		"markdown_content": fullMD,
		"completed_at":     doneAt,
		"error":            nil,
	})
	if err := s.Repo.SaveMeta(p.reportID, finalMeta); err != nil {
		s.failRun(p, err)
		return
	}
	_ = s.Repo.SaveProgress(p.reportID, map[string]any{
		"status":             "completed",
		"progress":           100,
		"message":            "Report generation completed",
		"current_section":    nil,
		"completed_sections": completed,
		"updated_at":         doneAt,
	})

	log("report_complete", "completed", map[string]any{
		"total_sections":     total,
		"total_time_seconds": math.Round(time.Since(start).Seconds()*100) / 100,
		"message":            "Report generation completed",
	}, nil, nil)
	s.console(p.reportID, "Report complete")

	_ = s.Tasks.CompleteTask(ctx, p.taskID, map[string]any{
		"report_id":     p.reportID,
		"simulation_id": p.simulationID,
		"status":        "completed",
	})
}

func (s *Service) failRun(p runParams, err error) {
	msg := err.Error()
	ctx := context.Background()
	log := func() {
		m := map[string]any{
			"timestamp":     time.Now().Format(time.RFC3339Nano),
			"report_id":     p.reportID,
			"action":        "error",
			"stage":         "failed",
			"details":       map[string]any{"error": msg, "message": msg},
			"section_title": nil,
			"section_index": nil,
		}
		_ = s.Repo.AppendAgentLog(p.reportID, m)
	}
	log()
	s.console(p.reportID, "ERROR: "+msg)
	_ = s.Repo.SaveProgress(p.reportID, map[string]any{
		"status":     "failed",
		"progress":   0,
		"message":    msg,
		"updated_at": time.Now().Format(time.RFC3339Nano),
	})
	meta, mErr := s.Repo.LoadMeta(p.reportID)
	if mErr == nil && meta != nil {
		meta["status"] = "failed"
		meta["error"] = msg
		_ = s.Repo.SaveMeta(p.reportID, meta)
	}
	_ = s.Tasks.FailTask(ctx, p.taskID, msg)
}

func mergeMeta(base map[string]any, patch map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(patch))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range patch {
		out[k] = v
	}
	return out
}

func normalizeOutlineMap(raw map[string]any, fallbackReq string) map[string]any {
	title := strings.TrimSpace(asString(raw["title"]))
	summary := strings.TrimSpace(asString(raw["summary"]))
	if title == "" {
		title = "Simulation analysis report"
	}
	if summary == "" {
		summary = fallbackReq
	}
	var sections []any
	if arr, ok := raw["sections"].([]any); ok {
		for _, e := range arr {
			m, ok := e.(map[string]any)
			if !ok {
				continue
			}
			t := strings.TrimSpace(asString(m["title"]))
			if t != "" {
				sections = append(sections, map[string]any{"title": t, "content": ""})
			}
		}
	}
	if len(sections) == 0 {
		sections = []any{
			map[string]any{"title": "Context and methodology", "content": ""},
			map[string]any{"title": "Key entities and relationships", "content": ""},
			map[string]any{"title": "Dynamics and findings", "content": ""},
			map[string]any{"title": "Conclusion", "content": ""},
		}
	}
	if len(sections) > 12 {
		sections = sections[:12]
	}
	return map[string]any{"title": title, "summary": summary, "sections": sections}
}

func compactPlanningContext(stats, search map[string]any) map[string]any {
	facts := []string{}
	switch x := search["facts"].(type) {
	case []any:
		for _, f := range x {
			facts = append(facts, asString(f))
			if len(facts) >= 12 {
				break
			}
		}
	case []string:
		for i, f := range x {
			if i >= 12 {
				break
			}
			facts = append(facts, f)
		}
	}
	return map[string]any{
		"statistics":    stats,
		"sample_facts":  facts,
		"nodes_matched": coerceInt(search["total_count"]),
	}
}

func searchToText(search map[string]any) string {
	var b strings.Builder
	if q := asString(search["query"]); q != "" {
		fmt.Fprintf(&b, "Query: %s\n\n", q)
	}
	switch facts := search["facts"].(type) {
	case []any:
		for i, f := range facts {
			fmt.Fprintf(&b, "%d. %s\n", i+1, asString(f))
		}
	case []string:
		for i, f := range facts {
			fmt.Fprintf(&b, "%d. %s\n", i+1, f)
		}
	}
	nodes, _ := search["nodes"].([]any)
	if len(nodes) > 0 {
		b.WriteString("\n### Nodes\n")
		for i, x := range nodes {
			if i >= 20 {
				break
			}
			if n, ok := x.(map[string]any); ok {
				fmt.Fprintf(&b, "- %s (%s)\n", asString(n["name"]), asString(n["summary"]))
			}
		}
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n... [truncated]"
}

func outlineWithContents(repo ports.ReportRepository, reportID string, outline map[string]any) map[string]any {
	sections, _ := outline["sections"].([]any)
	out := make([]any, 0, len(sections))
	for i := range sections {
		sm, ok := sections[i].(map[string]any)
		if !ok {
			continue
		}
		idx := i + 1
		md, _ := repo.ReadSectionMarkdown(reportID, idx)
		body := md
		if strings.HasPrefix(body, "## ") {
			lines := strings.SplitN(body, "\n", 3)
			if len(lines) >= 3 {
				body = strings.TrimSpace(lines[2])
			}
		}
		cp := map[string]any{
			"title":   asString(sm["title"]),
			"content": strings.TrimSpace(body),
		}
		out = append(out, cp)
	}
	return map[string]any{
		"title":    outline["title"],
		"summary":  outline["summary"],
		"sections": out,
	}
}
