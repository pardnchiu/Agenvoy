// * generate by claude opus 4.7
package file

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func readCSVHandler(path string, offset, limit int) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("os.Stat: %w", err)
	}
	if info.Size() > maxReadSize {
		return "", fmt.Errorf("file too large (%d bytes, max 1 MB)", info.Size())
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("os.ReadFile: %w", err)
	}
	raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})

	reader := csv.NewReader(bytes.NewReader(raw))
	if strings.ToLower(filepath.Ext(path)) == ".tsv" {
		reader.Comma = '\t'
	}
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Sprintf("%s is empty", path), nil
		}
		return "", fmt.Errorf("csv.Read header: %w", err)
	}

	skip := offset - 1
	if skip < 0 {
		skip = 0
	}

	rows := make([][]string, 0, limit+1)
	rows = append(rows, header)
	dataCount := 0
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("csv.Read row %d: %w", dataCount+1, err)
		}
		dataCount++
		if dataCount <= skip {
			continue
		}
		if len(rows)-1 >= limit {
			break
		}
		rows = append(rows, normalizeRow(record, len(header)))
	}

	if dataCount == 0 {
		return marshalRows(rows)
	}
	if len(rows) == 1 {
		return fmt.Sprintf("offset %d exceeds data rows %d in %s", offset, dataCount, path), nil
	}

	return marshalRows(rows)
}

func normalizeRow(row []string, cols int) []string {
	if len(row) == cols {
		return row
	}
	if len(row) > cols {
		return row[:cols]
	}
	out := make([]string, cols)
	copy(out, row)
	return out
}

func marshalRows(rows [][]string) (string, error) {
	b, err := json.Marshal(rows)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}
	return string(b), nil
}
