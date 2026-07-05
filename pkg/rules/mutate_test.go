package rules_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetReplacesScalar(t *testing.T) {
	live := map[string]interface{}{"action": "allow"}
	ch := secSchema(t).Set(live, "action", "deny")
	require.NotNil(t, ch)
	require.Equal(t, "allow", ch.Old)
	require.Equal(t, "deny", ch.New)
	require.Equal(t, "deny", live["action"])

	require.Nil(t, secSchema(t).Set(live, "action", "deny")) // no-op second time
}

func TestAddAppendsOnlyMissing(t *testing.T) {
	live := map[string]interface{}{"tag": []interface{}{"legacy"}}
	ch := secSchema(t).Add(live, "tag", []string{"legacy", "reviewed"})
	require.NotNil(t, ch)
	require.ElementsMatch(t, []interface{}{"legacy", "reviewed"}, live["tag"])

	require.Nil(t, secSchema(t).Add(live, "tag", []string{"reviewed"})) // already present
}

func TestRemoveDropsValues(t *testing.T) {
	live := map[string]interface{}{"tag": []interface{}{"legacy", "reviewed"}}
	ch := secSchema(t).Remove(live, "tag", []string{"legacy"})
	require.NotNil(t, ch)
	require.Equal(t, []interface{}{"reviewed"}, live["tag"])

	require.Nil(t, secSchema(t).Remove(live, "tag", []string{"absent"})) // nothing to remove
}
