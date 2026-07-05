package rules_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"scmbulk/pkg/rules"
)

func TestSchemaForSecurity(t *testing.T) {
	s, err := rules.SchemaFor("security")
	require.NoError(t, err)
	require.Equal(t, "security", s.Type)
	require.Equal(t, "/config/security/v1/security-rules", s.ResourcePath)
	require.True(t, s.IsListField("tag"))
	require.False(t, s.IsListField("action"))
}

func TestSchemaForUnknownErrors(t *testing.T) {
	_, err := rules.SchemaFor("bogus")
	require.Error(t, err)
	require.Contains(t, err.Error(), "bogus")
}

func TestSecuritySchemaToRowAndApply(t *testing.T) {
	s, _ := rules.SchemaFor("security")
	live := map[string]interface{}{"id": "abc", "action": "allow", "tag": []interface{}{"legacy"}}
	require.Equal(t, "allow", s.ToRow(live)["action"])
	changes := s.ApplyRow(live, map[string]string{"id": "abc", "action": "deny"})
	require.Len(t, changes, 1)
	require.Equal(t, "deny", live["action"])
}

func TestDecryptionSchema(t *testing.T) {
	s, err := rules.SchemaFor("decryption")
	require.NoError(t, err)
	require.Equal(t, "/config/security/v1/decryption-rules", s.ResourcePath)
	require.True(t, s.IsListField("category"))
	require.False(t, s.IsListField("action"))

	// action scalar + profile scalar round-trip
	live := map[string]interface{}{
		"id":     "d1",
		"action": "no-decrypt",
		"profile": "best-practice",
		"category": []interface{}{"URL_Exc"},
		"type":   map[string]interface{}{"ssl_forward_proxy": map[string]interface{}{}},
	}
	row := s.ToRow(live)
	require.Equal(t, "no-decrypt", row["action"])
	require.Equal(t, "best-practice", row["profile"])
	require.Equal(t, "URL_Exc", row["category"])
	require.Equal(t, "ssl_forward_proxy", row["type"]) // read-only summary

	// action is editable
	changes := s.ApplyRow(live, map[string]string{"id": "d1", "action": "decrypt"})
	require.Len(t, changes, 1)
	require.Equal(t, "decrypt", live["action"])

	// type is read-only: editing its cell is ignored (no change, not written)
	live2 := map[string]interface{}{"id": "d1", "type": map[string]interface{}{"ssl_forward_proxy": map[string]interface{}{}}}
	require.Empty(t, s.ApplyRow(live2, map[string]string{"id": "d1", "type": "ssh_proxy"}))
	_, stillFwd := live2["type"].(map[string]interface{})["ssl_forward_proxy"]
	require.True(t, stillFwd, "type must not be modified from its cell")
}
