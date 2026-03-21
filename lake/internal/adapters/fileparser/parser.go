package fileparser

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

// Parser extracts text from pdf/md/txt like backend FileParser.
type Parser struct{}

func New() *Parser { return &Parser{} }

var allowed = map[string]struct{}{
	".pdf": {}, ".md": {}, ".markdown": {}, ".txt": {},
}

func (Parser) ExtractText(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("file does not exist: %s", path)
		}
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("not a file: %s", path)
	}
	ext := strings.ToLower(filepath.Ext(path))
	if _, ok := allowed[ext]; !ok {
		return "", fmt.Errorf("unsupported file format: %s", ext)
	}
	switch ext {
	case ".pdf":
		return extractPDF(path)
	case ".md", ".markdown", ".txt":
		return readTextWithFallback(path)
	default:
		return "", fmt.Errorf("cannot handle file format: %s", ext)
	}
}

func extractPDF(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", fmt.Errorf("pdf open: %w", err)
	}
	defer f.Close()
	plain, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("pdf text: %w", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, plain); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func readTextWithFallback(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(bytes.ToValidUTF8(data, []byte{})), nil
}
