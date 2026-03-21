package reportstore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mirofish-offline/lake/internal/ports"
)

// Store implements ports.ReportRepository on disk (Python ReportManager paths).
type Store struct {
	root string
}

// New returns a store rooted at uploads/reports (pass cfg.ReportsDir()).
func New(reportsDir string) (*Store, error) {
	if reportsDir == "" {
		return nil, fmt.Errorf("reports dir empty")
	}
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		return nil, err
	}
	return &Store{root: reportsDir}, nil
}

var _ ports.ReportRepository = (*Store)(nil)

func (s *Store) reportDir(id string) string {
	return filepath.Join(s.root, id)
}

func (s *Store) EnsureFolder(reportID string) error {
	return os.MkdirAll(s.reportDir(reportID), 0o755)
}

func (s *Store) SaveMeta(reportID string, meta map[string]any) error {
	if err := s.EnsureFolder(reportID); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.reportDir(reportID), "meta.json"), meta)
}

func (s *Store) LoadMeta(reportID string) (map[string]any, error) {
	p := filepath.Join(s.reportDir(reportID), "meta.json")
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ports.ErrReportNotFound
		}
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Store) SaveProgress(reportID string, progress map[string]any) error {
	if err := s.EnsureFolder(reportID); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.reportDir(reportID), "progress.json"), progress)
}

func (s *Store) LoadProgress(reportID string) (map[string]any, error) {
	p := filepath.Join(s.reportDir(reportID), "progress.json")
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Store) SaveOutline(reportID string, outline map[string]any) error {
	if err := s.EnsureFolder(reportID); err != nil {
		return err
	}
	return writeJSON(filepath.Join(s.reportDir(reportID), "outline.json"), outline)
}

func (s *Store) SaveSection(reportID string, sectionIndex int, title, body string) error {
	if err := s.EnsureFolder(reportID); err != nil {
		return err
	}
	fn := fmt.Sprintf("section_%02d.md", sectionIndex)
	md := "## " + title + "\n\n"
	if strings.TrimSpace(body) != "" {
		md += strings.TrimSpace(body) + "\n\n"
	}
	return os.WriteFile(filepath.Join(s.reportDir(reportID), fn), []byte(md), 0o644)
}

func (s *Store) ReadSectionMarkdown(reportID string, sectionIndex int) (string, error) {
	p := filepath.Join(s.reportDir(reportID), fmt.Sprintf("section_%02d.md", sectionIndex))
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

func (s *Store) SaveFullMarkdown(reportID, markdown string) error {
	if err := s.EnsureFolder(reportID); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.reportDir(reportID), "full_report.md"), []byte(markdown), 0o644)
}

func (s *Store) LoadFullMarkdown(reportID string) (string, error) {
	b, err := os.ReadFile(filepath.Join(s.reportDir(reportID), "full_report.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(b), nil
}

func (s *Store) AppendAgentLog(reportID string, entry map[string]any) error {
	if err := s.EnsureFolder(reportID); err != nil {
		return err
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(s.reportDir(reportID), "agent_log.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

func (s *Store) AppendConsoleLog(reportID, line string) error {
	if err := s.EnsureFolder(reportID); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(s.reportDir(reportID), "console_log.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	_, err = f.WriteString(line)
	return err
}

func (s *Store) ReadAgentLog(reportID string, fromLine int) (logs []map[string]any, totalLines, fromLineOut int, hasMore bool) {
	fromLineOut = fromLine
	p := filepath.Join(s.reportDir(reportID), "agent_log.jsonl")
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, 0, fromLine, false
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, 0, fromLine, false
	}
	totalLines = len(lines)
	for i := fromLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var m map[string]any
		if json.Unmarshal([]byte(line), &m) == nil {
			logs = append(logs, m)
		}
	}
	return logs, totalLines, fromLine, false
}

func (s *Store) ReadConsoleLog(reportID string, fromLine int) (logs []string, totalLines, fromLineOut int, hasMore bool) {
	fromLineOut = fromLine
	p := filepath.Join(s.reportDir(reportID), "console_log.txt")
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, 0, fromLine, false
	}
	raw := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(raw) == 1 && raw[0] == "" {
		return nil, 0, fromLine, false
	}
	totalLines = len(raw)
	for i := fromLine; i < len(raw); i++ {
		logs = append(logs, strings.TrimRight(raw[i], "\r"))
	}
	return logs, totalLines, fromLine, false
}

func (s *Store) AssembleFullMarkdown(reportID string, outline map[string]any) (string, error) {
	title := str(outline["title"])
	summary := str(outline["summary"])
	dir := s.reportDir(reportID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var idx []int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "section_") && strings.HasSuffix(name, ".md") {
			mid := strings.TrimSuffix(strings.TrimPrefix(name, "section_"), ".md")
			n, err := strconv.Atoi(mid)
			if err == nil {
				idx = append(idx, n)
			}
		}
	}
	sort.Ints(idx)
	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(title)
	sb.WriteString("\n\n> ")
	sb.WriteString(summary)
	sb.WriteString("\n\n---\n\n")
	for _, i := range idx {
		b, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("section_%02d.md", i)))
		if err != nil {
			continue
		}
		sb.WriteString(string(b))
	}
	return sb.String(), nil
}

func (s *Store) LatestReportBySimulation(_ context.Context, simulationID string) (string, map[string]any, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return "", nil, err
	}
	var bestID string
	var bestMeta map[string]any
	var bestCreated string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		rid := e.Name()
		meta, err := s.LoadMeta(rid)
		if err != nil {
			continue
		}
		if str(meta["simulation_id"]) != simulationID {
			continue
		}
		created := str(meta["created_at"])
		if bestMeta == nil || created > bestCreated {
			bestCreated = created
			bestID = rid
			bestMeta = meta
		}
	}
	if bestMeta == nil {
		return "", nil, ports.ErrReportNotFound
	}
	return bestID, bestMeta, nil
}

func (s *Store) DeleteReport(reportID string) error {
	return os.RemoveAll(s.reportDir(reportID))
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
