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

func TestApplyRowBooleanCaseInsensitive(t *testing.T) {
	// Excel commonly exports booleans as TRUE/FALSE; these must be detected
	// as changes and applied as real bools so the PUT matches the preview.
	live := map[string]interface{}{
		"id":       "abc",
		"disabled": false,
	}
	row := map[string]string{
		"id":       "abc",
		"disabled": "TRUE",
	}
	changes := rules.ApplyRow(live, row)

	require.Len(t, changes, 1)
	require.Equal(t, "disabled", changes[0].Field)
	require.Equal(t, true, live["disabled"])
}

func TestApplyRowProfileSettingClear(t *testing.T) {
	// Clearing profile_setting must actually drop the group so the live object
	// re-serializes to the same empty cell the user typed (preview == write).
	live := map[string]interface{}{
		"id":              "abc",
		"profile_setting": map[string]interface{}{"group": []interface{}{"Best-Practice"}},
	}
	row := map[string]string{
		"id":              "abc",
		"profile_setting": "",
	}
	changes := rules.ApplyRow(live, row)

	require.Len(t, changes, 1)
	require.Equal(t, "profile_setting", changes[0].Field)
	require.Equal(t, "", rules.ToRow(live)["profile_setting"])
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
