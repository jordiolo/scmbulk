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
