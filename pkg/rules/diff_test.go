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
	changes, err := secSchema(t).ApplyRow(live, row)
	require.NoError(t, err)

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
	changes, err := secSchema(t).ApplyRow(live, row)
	require.NoError(t, err)

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
	changes, err := secSchema(t).ApplyRow(live, row)
	require.NoError(t, err)

	require.Len(t, changes, 1)
	require.Equal(t, "profile_setting", changes[0].Field)
	require.Equal(t, "", secSchema(t).ToRow(live)["profile_setting"])
}

func TestApplyRowClearScalarDeletesKey(t *testing.T) {
	// Emptying a scalar cell must delete the key (clear via omit under the
	// full-replace PUT), not set an empty string that the API may reject
	// (e.g. description "not allowed to be empty").
	live := map[string]interface{}{
		"id":          "abc",
		"action":      "allow",
		"description": "Comment",
	}
	row := map[string]string{
		"id":          "abc",
		"action":      "allow",
		"description": "",
	}
	changes, err := secSchema(t).ApplyRow(live, row)
	require.NoError(t, err)

	require.Len(t, changes, 1)
	require.Equal(t, "description", changes[0].Field)
	require.Equal(t, "Comment", changes[0].Old)
	require.Equal(t, "", changes[0].New)
	_, has := live["description"]
	require.False(t, has, "empty scalar cell should delete the key, not set \"\"")
}

func TestApplyRowAbsentScalarNoSpuriousChange(t *testing.T) {
	// A rule with no description and an empty CSV cell must not report a change.
	live := map[string]interface{}{"id": "abc", "action": "allow"}
	row := map[string]string{"id": "abc", "action": "allow", "description": ""}
	changes, err := secSchema(t).ApplyRow(live, row)
	require.NoError(t, err)
	require.Empty(t, changes)
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
	changes, err := secSchema(t).ApplyRow(live, row)
	require.NoError(t, err)
	require.Empty(t, changes)
}

func TestApplyRowProfileSettingUnrecognizedValueErrors(t *testing.T) {
	// Unrecognized profile_setting cell (missing "group:" prefix) must error,
	// not silently clear the field.
	live := map[string]interface{}{
		"id":              "abc",
		"profile_setting": map[string]interface{}{"group": []interface{}{"Best-Practice"}},
	}
	row := map[string]string{
		"id":              "abc",
		"profile_setting": "best-practice", // missing "group:" prefix
	}
	changes, err := secSchema(t).ApplyRow(live, row)
	require.Error(t, err)
	require.Contains(t, err.Error(), "profile_setting")
	require.Contains(t, err.Error(), "best-practice")
	require.Nil(t, changes)
	// Live object must remain unchanged (not deleted).
	require.Equal(t, map[string]interface{}{"group": []interface{}{"Best-Practice"}}, live["profile_setting"])
}
