package rules

// FieldChange records one field's old and new CSV-cell values.
type FieldChange struct {
	Field string
	Old   string
	New   string
}

// ApplyRow applies the edited CSV row onto the live object, changing only the
// editable columns whose value differs. It returns the list of changes.
func ApplyRow(live map[string]interface{}, row map[string]string) []FieldChange {
	current := ToRow(live)
	var changes []FieldChange
	for _, col := range Columns {
		if col == "id" || col == "position" {
			continue
		}
		newCell, present := row[col]
		if !present {
			continue
		}
		if normalizeCell(col, newCell) == normalizeCell(col, current[col]) {
			continue
		}
		changes = append(changes, FieldChange{Field: col, Old: current[col], New: newCell})
		setField(live, col, newCell)
	}
	return changes
}
