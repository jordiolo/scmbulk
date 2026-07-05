// Package selection compiles the mode B rule filter (names_file + match).
package selection

import (
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"strings"

	"scmbulk/pkg/config"
)

type matchOp int

const (
	opAny matchOp = iota // bare list, {any:}, or a single scalar value
	opAll                // {all:}
)

type fieldPred struct {
	field  string
	op     matchOp
	values []string
}

// Filter selects rules by an optional name list AND optional field predicates.
type Filter struct {
	names     map[string]bool // nil = no name filter
	nameRegex *regexp.Regexp
	preds     []fieldPred
}

// New builds a Filter from the config selection block.
func New(sel config.Selection) (*Filter, error) {
	f := &Filter{}
	for key, raw := range sel.Match {
		if key == "name_regex" {
			s, ok := raw.(string)
			if !ok {
				return nil, fmt.Errorf("match.name_regex must be a string")
			}
			re, err := regexp.Compile(s)
			if err != nil {
				return nil, fmt.Errorf("invalid name_regex: %w", err)
			}
			f.nameRegex = re
			continue
		}
		pred, err := buildPred(key, raw)
		if err != nil {
			return nil, err
		}
		f.preds = append(f.preds, pred)
	}
	if sel.NamesFile != "" {
		names, err := loadNames(sel.NamesFile)
		if err != nil {
			return nil, err
		}
		f.names = names
	}
	return f, nil
}

func buildPred(field string, raw interface{}) (fieldPred, error) {
	switch v := raw.(type) {
	case []interface{}:
		vals, err := toListStrings(field, v)
		if err != nil {
			return fieldPred{}, err
		}
		return fieldPred{field: field, op: opAny, values: vals}, nil
	case map[string]interface{}:
		if a, ok := v["all"]; ok {
			vals, err := toListStrings(field, a)
			if err != nil {
				return fieldPred{}, err
			}
			return fieldPred{field: field, op: opAll, values: vals}, nil
		}
		if a, ok := v["any"]; ok {
			vals, err := toListStrings(field, a)
			if err != nil {
				return fieldPred{}, err
			}
			return fieldPred{field: field, op: opAny, values: vals}, nil
		}
		return fieldPred{}, fmt.Errorf("match.%s: expected \"all\" or \"any\"", field)
	default: // scalar (string/bool/number)
		s := strings.TrimSpace(fmt.Sprintf("%v", v))
		if s == "" {
			return fieldPred{}, fmt.Errorf("match.%s: empty value", field)
		}
		return fieldPred{field: field, op: opAny, values: []string{s}}, nil
	}
}

func toListStrings(field string, raw interface{}) ([]string, error) {
	list, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("match.%s: expected a list of values", field)
	}
	var out []string
	for _, e := range list {
		s := strings.TrimSpace(fmt.Sprintf("%v", e))
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("match.%s: empty value list", field)
	}
	return out, nil
}

// Matches reports whether a live rule passes all configured criteria (AND).
func (f *Filter) Matches(rule map[string]interface{}) bool {
	name, _ := rule["name"].(string)
	if f.names != nil && !f.names[name] {
		return false
	}
	if f.nameRegex != nil && !f.nameRegex.MatchString(name) {
		return false
	}
	for _, p := range f.preds {
		if !matchField(rule, p) {
			return false
		}
	}
	return true
}

func matchField(rule map[string]interface{}, p fieldPred) bool {
	raw, ok := rule[p.field]
	if !ok {
		return false // absent field never matches
	}
	if list, ok := raw.([]interface{}); ok { // list field -> contains
		set := make(map[string]bool, len(list))
		for _, e := range list {
			set[fmt.Sprintf("%v", e)] = true
		}
		if p.op == opAll {
			for _, want := range p.values {
				if !set[want] {
					return false
				}
			}
			return true
		}
		for _, want := range p.values { // opAny
			if set[want] {
				return true
			}
		}
		return false
	}
	// scalar field -> equals (any of, when multiple values)
	s := fmt.Sprintf("%v", raw)
	for _, want := range p.values {
		if s == want {
			return true
		}
	}
	return false
}

func loadNames(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("reading names_file: %w", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing names_file: %w", err)
	}
	names := make(map[string]bool)
	for i, rec := range records {
		if len(rec) == 0 {
			continue
		}
		val := strings.TrimSpace(rec[0])
		if i == 0 && strings.EqualFold(val, "name") {
			continue // header
		}
		if val != "" {
			names[val] = true
		}
	}
	return names, nil
}
