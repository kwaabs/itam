// Package importer turns uploaded CSV/XLSX spreadsheets into header-keyed rows
// that the API can validate and commit. It is format-only: all domain
// validation (types, lookups, lifecycles) lives in the HTTP layer so the same
// metadata rules apply to imports and to single-record creates.
package importer

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Row is one parsed data row: header (lowercased) -> trimmed value, plus the
// 1-based source line so previews can point the user at the offending row.
type Row struct {
	Line   int               `json:"line"`
	Values map[string]string `json:"values"`
}

// Parse reads CSV or XLSX (detected by filename extension) into a header list
// and data rows. Header names are trimmed and lowercased for case-insensitive
// lookups. Rows whose first cell starts with '#' are treated as comments.
func Parse(filename string, data []byte) (headers []string, rows []Row, err error) {
	if strings.HasSuffix(strings.ToLower(filename), ".xlsx") {
		return parseXLSX(data)
	}
	return parseCSV(data)
}

func parseCSV(data []byte) ([]string, []Row, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("parse csv: %w", err)
	}
	return fromRecords(records)
}

func parseXLSX(data []byte) ([]string, []Row, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, nil, fmt.Errorf("parse xlsx: %w", err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, nil, fmt.Errorf("workbook has no sheets")
	}
	records, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, nil, fmt.Errorf("read sheet %q: %w", sheets[0], err)
	}
	return fromRecords(records)
}

// fromRecords takes raw rows (first non-comment row = header) and builds Rows.
// The 1-based Line tracks the original spreadsheet row including header/comments.
func fromRecords(records [][]string) ([]string, []Row, error) {
	var headers []string
	var rows []Row
	for i, rec := range records {
		line := i + 1
		if isComment(rec) {
			continue
		}
		if headers == nil {
			headers = make([]string, len(rec))
			for j, h := range rec {
				headers[j] = strings.ToLower(strings.TrimSpace(h))
			}
			continue
		}
		vals := map[string]string{}
		nonEmpty := false
		for j, h := range headers {
			if h == "" {
				continue
			}
			v := ""
			if j < len(rec) {
				v = strings.TrimSpace(rec[j])
			}
			vals[h] = v
			if v != "" {
				nonEmpty = true
			}
		}
		if !nonEmpty {
			continue
		}
		rows = append(rows, Row{Line: line, Values: vals})
	}
	if headers == nil {
		return nil, nil, fmt.Errorf("no header row found")
	}
	return headers, rows, nil
}

func isComment(rec []string) bool {
	return len(rec) > 0 && strings.HasPrefix(strings.TrimSpace(rec[0]), "#")
}
