// Package template renders per-rule Go text/templates for mode B change values.
package template

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// Render executes tmpl with the rule map as data. Templates without an action
// delimiter are returned verbatim (cheap path for literal values).
func Render(tmpl string, rule map[string]interface{}) (string, error) {
	if !strings.Contains(tmpl, "{{") {
		return tmpl, nil
	}
	t, err := template.New("value").Funcs(funcs).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parsing template %q: %w", tmpl, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, rule); err != nil {
		return "", fmt.Errorf("rendering template %q: %w", tmpl, err)
	}
	return buf.String(), nil
}

var funcs = template.FuncMap{
	"has":      has,
	"contains": strings.Contains,
	"lower":    strings.ToLower,
	"upper":    strings.ToUpper,
	"replace":  func(s, old, neu string) string { return strings.ReplaceAll(s, old, neu) },
	"join":     func(list interface{}, sep string) string { return strings.Join(toStrings(list), sep) },
	"split":    func(s, sep string) []string { return strings.Split(s, sep) },
}

func has(list interface{}, value string) bool {
	for _, e := range toStrings(list) {
		if e == value {
			return true
		}
	}
	return false
}

func toStrings(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, e := range t {
			out = append(out, fmt.Sprintf("%v", e))
		}
		return out
	case nil:
		return nil
	default:
		return []string{fmt.Sprintf("%v", t)}
	}
}
