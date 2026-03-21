package ports

import (
	"context"
	"errors"
)

// ErrReportNotFound is returned when a report folder or meta.json is missing.
var ErrReportNotFound = errors.New("report not found")

// ReportRepository persists report artifacts (Python ReportManager layout under uploads/reports/{id}/).
type ReportRepository interface {
	EnsureFolder(reportID string) error
	SaveMeta(reportID string, meta map[string]any) error
	LoadMeta(reportID string) (map[string]any, error)
	SaveProgress(reportID string, progress map[string]any) error
	LoadProgress(reportID string) (map[string]any, error)
	SaveOutline(reportID string, outline map[string]any) error
	SaveSection(reportID string, sectionIndex int, title, body string) error
	ReadSectionMarkdown(reportID string, sectionIndex int) (string, error)
	SaveFullMarkdown(reportID, markdown string) error
	LoadFullMarkdown(reportID string) (string, error)
	AppendAgentLog(reportID string, entry map[string]any) error
	AppendConsoleLog(reportID, line string) error
	ReadAgentLog(reportID string, fromLine int) (logs []map[string]any, totalLines int, fromLineOut int, hasMore bool)
	ReadConsoleLog(reportID string, fromLine int) (logs []string, totalLines int, fromLineOut int, hasMore bool)
	AssembleFullMarkdown(reportID string, outline map[string]any) (string, error)
	LatestReportBySimulation(ctx context.Context, simulationID string) (reportID string, meta map[string]any, err error)
	ListReports(ctx context.Context, simulationID string, limit int) ([]map[string]any, error)
	ListGeneratedSections(reportID string) ([]map[string]any, error)
	DeleteReport(reportID string) error
}
