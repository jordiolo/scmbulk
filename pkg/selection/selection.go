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

// Filter selects rules by an optional name list AND optional match conditions.
type Filter struct {
	names     map[string]bool // nil = no name filter
	action    string
	tag       string
	nameRegex *regexp.Regexp
}

// New builds a Filter from the config selection block.
func New(sel config.Selection) (*Filter, error) {
	f := &Filter{action: sel.Match.Action, tag: sel.Match.Tag}
	if sel.Match.NameRegex != "" {
		re, err := regexp.Compile(sel.Match.NameRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid name_regex: %w", err)
		}
		f.nameRegex = re
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

// Matches reports whether a live rule passes all configured criteria (AND).
func (f *Filter) Matches(rule map[string]interface{}) bool {
	name, _ := rule["name"].(string)
	if f.names != nil && !f.names[name] {
		return false
	}
	if f.action != "" {
		if a, _ := rule["action"].(string); a != f.action {
			return false
		}
	}
	if f.tag != "" && !hasTag(rule, f.tag) {
		return false
	}
	if f.nameRegex != nil && !f.nameRegex.MatchString(name) {
		return false
	}
	return true
}

func hasTag(rule map[string]interface{}, tag string) bool {
	list, ok := rule["tag"].([]interface{})
	if !ok {
		return false
	}
	for _, t := range list {
		if fmt.Sprintf("%v", t) == tag {
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
