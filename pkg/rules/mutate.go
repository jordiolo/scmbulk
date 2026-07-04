package rules

// Set replaces field with value. Returns the change, or nil if unchanged.
func Set(live map[string]interface{}, field, value string) *FieldChange {
	old := cellFromValue(field, live[field])
	if normalizeCell(field, old) == normalizeCell(field, value) {
		return nil
	}
	setField(live, field, value)
	return &FieldChange{Field: field, Old: old, New: cellFromValue(field, live[field])}
}

// Add appends the missing values to a list field. No-op on non-list fields.
func Add(live map[string]interface{}, field string, values []string) *FieldChange {
	if !listFields[field] {
		return nil
	}
	old := cellFromValue(field, live[field])
	cur := toStringSlice(live[field])
	seen := make(map[string]bool, len(cur))
	for _, v := range cur {
		seen[v] = true
	}
	changed := false
	for _, v := range values {
		if !seen[v] {
			cur = append(cur, v)
			seen[v] = true
			changed = true
		}
	}
	if !changed {
		return nil
	}
	live[field] = toIfaceSlice(cur)
	return &FieldChange{Field: field, Old: old, New: cellFromValue(field, live[field])}
}

// Remove drops the given values from a list field. No-op on non-list fields.
func Remove(live map[string]interface{}, field string, values []string) *FieldChange {
	if !listFields[field] {
		return nil
	}
	old := cellFromValue(field, live[field])
	drop := make(map[string]bool, len(values))
	for _, v := range values {
		drop[v] = true
	}
	var kept []string
	changed := false
	for _, v := range toStringSlice(live[field]) {
		if drop[v] {
			changed = true
			continue
		}
		kept = append(kept, v)
	}
	if !changed {
		return nil
	}
	live[field] = toIfaceSlice(kept)
	return &FieldChange{Field: field, Old: old, New: cellFromValue(field, live[field])}
}
