package file

import (
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

func readPDF(absPath string, offset, limit int) (string, error) {
	file, reader, err := pdf.Open(absPath)
	if err != nil {
		return "", fmt.Errorf("pdf.Open: %w", err)
	}
	defer file.Close()

	total := reader.NumPage()
	start := max(offset, 1)
	end := total
	if limit > 0 && start+limit-1 < end {
		end = start + limit - 1
	}

	var sb strings.Builder
	for i := start; i <= end; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(text)
	}

	if sb.Len() == 0 {
		return "", fmt.Errorf("no text extracted from PDF: %s", absPath)
	}
	return sb.String(), nil
}
