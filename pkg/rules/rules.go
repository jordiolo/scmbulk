// Package rules owns the rule model, CSV serialization, the mode A diff and the
// mode B list mutations, parameterized per rule type by Schema. It has no
// knowledge of HTTP.
package rules

import (
	"encoding/csv"
	"os"
	"strings"
)

const listSep = ";"

// --- shared, schema-independent helpers ---

func toStringSlice(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, e := range t {
			out = append(out, toScalarString(e))
		}
		return out
	default:
		return nil
	}
}

func toScalarString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return strings_Sprint(v)
}

func splitList(s, sep string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func toIfaceSlice(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// WriteCSV writes rows using the schema's column order as the header.
func (s *Schema) WriteCSV(path string, rows []map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(s.columns); err != nil {
		return err
	}
	for _, row := range rows {
		rec := make([]string, len(s.columns))
		for i, col := range s.columns {
			rec[i] = row[col]
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// ReadCSV reads a CSV whose header names the columns, into cell maps. It is
// schema-independent (keys come from the file header).
func ReadCSV(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	header := records[0]
	var rows []map[string]string
	for _, rec := range records[1:] {
		row := make(map[string]string, len(header))
		for i, h := range header {
			if i < len(rec) {
				row[h] = rec[i]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}
