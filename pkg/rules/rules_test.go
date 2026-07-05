package rules_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"scmbulk/pkg/rules"
)

func secSchema(t *testing.T) *rules.Schema {
	t.Helper()
	s, err := rules.SchemaFor("security")
	require.NoError(t, err)
	return s
}

func TestToRowSerializesScalarsListsAndBools(t *testing.T) {
	obj := map[string]interface{}{
		"id":       "abc",
		"name":     "Rule-Web",
		"action":   "allow",
		"source":   []interface{}{"any"},
		"tag":      []interface{}{"legacy", "web"},
		"disabled": false,
		"profile_setting": map[string]interface{}{
			"group": []interface{}{"Best-Practice"},
		},
	}
	row := secSchema(t).ToRow(obj)
	require.Equal(t, "abc", row["id"])
	require.Equal(t, "allow", row["action"])
	require.Equal(t, "any", row["source"])
	require.Equal(t, "legacy;web", row["tag"])
	require.Equal(t, "false", row["disabled"])
	require.Equal(t, "group:Best-Practice", row["profile_setting"])
}

func TestNewEditableFieldsRoundTrip(t *testing.T) {
	// policy_type (scalar) and the HIP/devices string lists must serialize and
	// apply back like the other columns.
	obj := map[string]interface{}{
		"id":              "abc",
		"policy_type":     "Security",
		"source_hip":      []interface{}{"any"},
		"destination_hip": []interface{}{"hip-a", "hip-b"},
		"devices":         []interface{}{"any"},
	}
	row := secSchema(t).ToRow(obj)
	require.Equal(t, "Security", row["policy_type"])
	require.Equal(t, "any", row["source_hip"])
	require.Equal(t, "hip-a;hip-b", row["destination_hip"])
	require.Equal(t, "any", row["devices"])
	require.True(t, secSchema(t).IsListField("source_hip"))
	require.True(t, secSchema(t).IsListField("destination_hip"))
	require.True(t, secSchema(t).IsListField("devices"))
	require.False(t, secSchema(t).IsListField("policy_type"))

	// Editing them applies with the right types.
	live := map[string]interface{}{
		"id":          "abc",
		"policy_type": "Security",
		"source_hip":  []interface{}{"any"},
	}
	changes := secSchema(t).ApplyRow(live, map[string]string{
		"id":          "abc",
		"policy_type": "intrazone",
		"source_hip":  "hip-x;hip-y",
	})
	require.Len(t, changes, 2)
	require.Equal(t, "intrazone", live["policy_type"])
	require.ElementsMatch(t, []interface{}{"hip-x", "hip-y"}, live["source_hip"])
}

func TestWriteThenReadCSVRoundTrips(t *testing.T) {
	rows := []map[string]string{
		{"id": "1", "name": "r1", "action": "allow", "tag": "legacy;web"},
		{"id": "2", "name": "r2", "action": "deny", "tag": ""},
	}
	path := filepath.Join(t.TempDir(), "out.csv")
	require.NoError(t, secSchema(t).WriteCSV(path, rows))

	got, err := rules.ReadCSV(path)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "r1", got[0]["name"])
	require.Equal(t, "legacy;web", got[0]["tag"])
	require.Equal(t, "deny", got[1]["action"])
}
