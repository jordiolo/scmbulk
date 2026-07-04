package rules_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"scmbulk/pkg/rules"
)

func TestSetReplacesScalar(t *testing.T) {
	live := map[string]interface{}{"action": "allow"}
	ch := rules.Set(live, "action", "deny")
	require.NotNil(t, ch)
	require.Equal(t, "allow", ch.Old)
	require.Equal(t, "deny", ch.New)
	require.Equal(t, "deny", live["action"])

	require.Nil(t, rules.Set(live, "action", "deny")) // no-op second time
}

func TestAddAppendsOnlyMissing(t *testing.T) {
	live := map[string]interface{}{"tag": []interface{}{"legacy"}}
	ch := rules.Add(live, "tag", []string{"legacy", "reviewed"})
	require.NotNil(t, ch)
	require.ElementsMatch(t, []interface{}{"legacy", "reviewed"}, live["tag"])

	require.Nil(t, rules.Add(live, "tag", []string{"reviewed"})) // already present
}

func TestRemoveDropsValues(t *testing.T) {
	live := map[string]interface{}{"tag": []interface{}{"legacy", "reviewed"}}
	ch := rules.Remove(live, "tag", []string{"legacy"})
	require.NotNil(t, ch)
	require.Equal(t, []interface{}{"reviewed"}, live["tag"])

	require.Nil(t, rules.Remove(live, "tag", []string{"absent"})) // nothing to remove
}
