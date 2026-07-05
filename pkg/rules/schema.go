package rules

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// cellCodec gives a field a bespoke CSV-cell encoding. fromCell's second
// return is false when the cell means "delete the key" (clear the field).
// The third return is an error if the cell value is invalid.
type cellCodec struct {
	toCell   func(v interface{}) string
	fromCell func(cell string) (interface{}, bool, error)
}

// Schema describes one rule type: its API resource, CSV columns, and how each
// field maps to/from a CSV cell.
type Schema struct {
	Type         string
	ResourcePath string
	columns      []string
	listFields   map[string]bool
	boolFields   map[string]bool
	// complexFields are shown read-only and never written from a cell.
	complexFields map[string]bool
	// special holds bespoke per-field codecs (e.g. security profile_setting).
	special map[string]cellCodec
}

// Columns returns the ordered CSV columns for this schema.
func (s *Schema) Columns() []string { return s.columns }

// IsListField reports whether col is a list-valued field in this schema.
func (s *Schema) IsListField(col string) bool { return s.listFields[col] }

func (s *Schema) cellFromValue(col string, v interface{}) string {
	if codec, ok := s.special[col]; ok {
		return codec.toCell(v)
	}
	if s.complexFields[col] {
		return complexToCell(v)
	}
	if v == nil {
		return ""
	}
	if s.listFields[col] {
		return strings.Join(toStringSlice(v), listSep)
	}
	if s.boolFields[col] {
		if b, ok := v.(bool); ok {
			return strconv.FormatBool(b)
		}
	}
	return fmt.Sprintf("%v", v)
}

func (s *Schema) setField(obj map[string]interface{}, col, cell string) error {
	if codec, ok := s.special[col]; ok {
		v, keep, err := codec.fromCell(cell)
		if err != nil {
			return err
		}
		if keep {
			obj[col] = v
		} else {
			delete(obj, col)
		}
		return nil
	}
	if s.complexFields[col] {
		return nil // read-only, never written from a cell
	}
	switch {
	case s.listFields[col]:
		obj[col] = toIfaceSlice(splitList(cell, listSep))
	case s.boolFields[col]:
		obj[col] = strings.EqualFold(strings.TrimSpace(cell), "true")
	default:
		if strings.TrimSpace(cell) == "" {
			delete(obj, col)
		} else {
			obj[col] = cell
		}
	}
	return nil
}

func (s *Schema) normalizeCell(col, cell string) string {
	if s.listFields[col] {
		parts := splitList(cell, listSep)
		sort.Strings(parts)
		return strings.Join(parts, listSep)
	}
	if s.boolFields[col] {
		return strings.ToLower(strings.TrimSpace(cell))
	}
	return strings.TrimSpace(cell)
}

// ToRow converts a full rule object into a flat CSV cell map for this schema.
func (s *Schema) ToRow(obj map[string]interface{}) map[string]string {
	row := make(map[string]string, len(s.columns))
	for _, col := range s.columns {
		row[col] = s.cellFromValue(col, obj[col])
	}
	return row
}

// complexToCell renders a nested-object field read-only as its sorted top-level
// keys (e.g. {"ssl_forward_proxy":{}} -> "ssl_forward_proxy").
func complexToCell(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, listSep)
}

// profileToCell/profileFromCell implement the security profile_setting codec.
func profileToCell(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}
	if g, ok := m["group"]; ok {
		return "group:" + strings.Join(toStringSlice(g), ",")
	}
	return ""
}

func profileFromCell(cell string) (interface{}, bool, error) {
	cell = strings.TrimSpace(cell)
	if cell == "" {
		return nil, false, nil // clear
	}
	if strings.HasPrefix(cell, "group:") {
		groups := splitList(strings.TrimPrefix(cell, "group:"), ",")
		return map[string]interface{}{"group": toIfaceSlice(groups)}, true, nil
	}
	return nil, false, fmt.Errorf("profile_setting: unrecognized value %q (expected \"group:<name>\" or empty)", cell)
}


var securitySchema = &Schema{
	Type:         "security",
	ResourcePath: "/config/security/v1/security-rules",
	columns: []string{
		"id", "position", "name", "description", "policy_type", "action", "from", "to",
		"source", "source_hip", "destination", "destination_hip", "source_user",
		"application", "service", "category", "tag", "log_setting", "log_start",
		"log_end", "disabled", "negate_source", "negate_destination",
		"profile_setting", "schedule", "devices",
	},
	listFields: map[string]bool{
		"from": true, "to": true, "source": true, "destination": true,
		"source_user": true, "application": true, "service": true, "category": true,
		"tag": true, "source_hip": true, "destination_hip": true, "devices": true,
	},
	boolFields: map[string]bool{
		"disabled": true, "negate_source": true, "negate_destination": true,
		"log_start": true, "log_end": true,
	},
	complexFields: map[string]bool{},
	special:       map[string]cellCodec{"profile_setting": {toCell: profileToCell, fromCell: profileFromCell}},
}

var decryptionSchema = &Schema{
	Type:         "decryption",
	ResourcePath: "/config/security/v1/decryption-rules",
	columns: []string{
		"id", "position", "name", "description", "action", "profile", "type",
		"from", "to", "source", "destination", "source_user", "service",
		"category", "source_hip", "destination_hip", "log_setting",
		"log_success", "log_fail", "disabled", "negate_source",
		"negate_destination", "tag",
	},
	listFields: map[string]bool{
		"from": true, "to": true, "source": true, "destination": true,
		"source_user": true, "service": true, "category": true,
		"source_hip": true, "destination_hip": true, "tag": true,
	},
	boolFields: map[string]bool{
		"disabled": true, "negate_source": true, "negate_destination": true,
		"log_success": true, "log_fail": true,
	},
	complexFields: map[string]bool{"type": true},
	special:       map[string]cellCodec{},
}

var schemaRegistry = map[string]*Schema{
	"security":   securitySchema,
	"decryption": decryptionSchema,
}

// SchemaFor returns the schema for a rule type, or an error for unknown types.
func SchemaFor(ruleType string) (*Schema, error) {
	s, ok := schemaRegistry[ruleType]
	if !ok {
		return nil, fmt.Errorf("unknown rule type %q (valid: security, decryption)", ruleType)
	}
	return s, nil
}
