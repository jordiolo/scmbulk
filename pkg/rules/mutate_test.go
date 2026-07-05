package rules_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"scmbulk/pkg/rules"
)

func decSchema(t *testing.T) *rules.Schema {
	t.Helper()
	s, err := rules.SchemaFor("decryption")
	require.NoError(t, err)
	return s
}

func TestSetReplacesScalar(t *testing.T) {
	live := map[string]interface{}{"action": "allow"}
	ch, err := secSchema(t).Set(live, "action", "deny")
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.Equal(t, "allow", ch.Old)
	require.Equal(t, "deny", ch.New)
	require.Equal(t, "deny", live["action"])

	ch2, err2 := secSchema(t).Set(live, "action", "deny")
	require.NoError(t, err2)
	require.Nil(t, ch2) // no-op second time
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

func TestSetIgnoresComplexFields(t *testing.T) {
	dec := decSchema(t)
	live := map[string]interface{}{
		"id":   "d1",
		"type": map[string]interface{}{"ssl_forward_proxy": map[string]interface{}{}},
	}
	ch, err := dec.Set(live, "type", "ssh_proxy")
	require.NoError(t, err)
	require.Nil(t, ch, "Set on a complex field must return nil (no change)")
	_, stillFwd := live["type"].(map[string]interface{})["ssl_forward_proxy"]
	require.True(t, stillFwd, "type must remain unchanged")
}
