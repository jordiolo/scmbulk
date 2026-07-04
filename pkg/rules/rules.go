// Package rules owns the security-rule model, CSV serialization, the mode A
// diff and the mode B list mutations. It has no knowledge of HTTP.
package rules

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Columns is the ordered set of CSV columns emitted and read back.
var Columns = []string{
	"id", "position", "name", "description", "action", "from", "to",
	"source", "destination", "source_user", "application", "service",
	"category", "tag", "log_setting", "log_start", "log_end", "disabled",
	"negate_source", "negate_destination", "profile_setting", "schedule",
}

const listSep = ";"

var listFields = map[string]bool{
	"from": true, "to": true, "source": true, "destination": true,
	"source_user": true, "application": true, "service": true,
	"category": true, "tag": true,
}

var boolFields = map[string]bool{
	"disabled": true, "negate_source": true, "negate_destination": true,
	"log_start": true, "log_end": true,
}

// IsListField reports whether col is a list-valued rule field.
func IsListField(col string) bool { return listFields[col] }

// ToRow converts a full rule object into a flat CSV cell map.
func ToRow(obj map[string]interface{}) map[string]string {
	row := make(map[string]string, len(Columns))
	for _, col := range Columns {
		row[col] = cellFromValue(col, obj[col])
	}
	return row
}

func cellFromValue(col string, v interface{}) string {
	if v == nil {
		return ""
	}
	if col == "profile_setting" {
		return profileToCell(v)
	}
	if listFields[col] {
		return strings.Join(toStringSlice(v), listSep)
	}
	if boolFields[col] {
		if b, ok := v.(bool); ok {
			return strconv.FormatBool(b)
		}
	}
	return fmt.Sprintf("%v", v)
}

func profileToCell(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}
	if g, ok := m["group"]; ok {
		return "group:" + strings.Join(toStringSlice(g), ",")
	}
	return "" // "profiles" form is left untouched in the raw object, not serialized
}

func toStringSlice(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, e := range t {
			out = append(out, fmt.Sprintf("%v", e))
		}
		return out
	default:
		return nil
	}
}

// setField writes a CSV cell value back into the rule object with the right type.
func setField(obj map[string]interface{}, col, cell string) {
	switch {
	case col == "profile_setting":
		if strings.HasPrefix(cell, "group:") {
			groups := splitList(strings.TrimPrefix(cell, "group:"), ",")
			obj[col] = map[string]interface{}{"group": toIfaceSlice(groups)}
		}
	case listFields[col]:
		obj[col] = toIfaceSlice(splitList(cell, listSep))
	case boolFields[col]:
		obj[col] = cell == "true"
	default:
		obj[col] = cell
	}
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

// normalizeCell canonicalizes a cell for comparison (trim; sort list members).
func normalizeCell(col, cell string) string {
	if listFields[col] {
		parts := splitList(cell, listSep)
		sort.Strings(parts)
		return strings.Join(parts, listSep)
	}
	return strings.TrimSpace(cell)
}

// WriteCSV writes rows using Columns as the header order.
func WriteCSV(path string, rows []map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(Columns); err != nil {
		return err
	}
	for _, row := range rows {
		rec := make([]string, len(Columns))
		for i, col := range Columns {
			rec[i] = row[col]
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

// ReadCSV reads a CSV whose header names the columns, into cell maps.
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
