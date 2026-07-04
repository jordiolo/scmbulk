package rules_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"scmbulk/pkg/rules"
)

func TestApplyRowChangesOnlyEditedFields(t *testing.T) {
	live := map[string]interface{}{
		"id":     "abc",
		"name":   "Rule-Web",
		"action": "allow",
		"tag":    []interface{}{"legacy"},
	}
	row := map[string]string{
		"id":     "abc",
		"name":   "Rule-Web",        // unchanged
		"action": "deny",            // changed
		"tag":    "legacy;reviewed", // changed
	}
	changes := rules.ApplyRow(live, row)

	require.Len(t, changes, 2)
	require.Equal(t, "deny", live["action"])
	require.ElementsMatch(t, []interface{}{"legacy", "reviewed"}, live["tag"])

	byField := map[string]rules.FieldChange{}
	for _, c := range changes {
		byField[c.Field] = c
	}
	require.Equal(t, "allow", byField["action"].Old)
	require.Equal(t, "deny", byField["action"].New)
}

func TestApplyRowNoChangesWhenEqual(t *testing.T) {
	live := map[string]interface{}{
		"id":     "abc",
		"action": "allow",
		"tag":    []interface{}{"web", "legacy"},
	}
	row := map[string]string{
		"id":     "abc",
		"action": "allow",
		"tag":    "legacy;web", // same set, different order -> no change
	}
	changes := rules.ApplyRow(live, row)
	require.Empty(t, changes)
}
