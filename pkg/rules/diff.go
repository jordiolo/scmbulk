package rules

// FieldChange records one field's old and new CSV-cell values.
type FieldChange struct {
	Field string
	Old   string
	New   string
}

// ApplyRow applies the edited CSV row onto the live object, changing only the
// editable columns whose value differs. It returns the list of changes and an
// error if any cell is invalid.
func (s *Schema) ApplyRow(live map[string]interface{}, row map[string]string) ([]FieldChange, error) {
	current := s.ToRow(live)
	var changes []FieldChange
	for _, col := range s.columns {
		if col == "id" || col == "position" || s.complexFields[col] {
			continue
		}
		newCell, present := row[col]
		if !present {
			continue
		}
		if s.normalizeCell(col, newCell) == s.normalizeCell(col, current[col]) {
			continue
		}
		// setField only errors via a field's codec, whose message already
		// names the field (e.g. profile_setting), so no extra wrapping here.
		if err := s.setField(live, col, newCell); err != nil {
			return nil, err
		}
		changes = append(changes, FieldChange{Field: col, Old: current[col], New: newCell})
	}
	return changes, nil
}
