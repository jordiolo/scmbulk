package template_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	tmpl "scmbulk/pkg/template"
)

func TestRenderConditionalOnScalar(t *testing.T) {
	rule := map[string]interface{}{"action": "allow"}
	out, err := tmpl.Render(`{{ if (eq .action "allow") }}deny{{ else }}drop{{ end }}`, rule)
	require.NoError(t, err)
	require.Equal(t, "deny", out)
}

func TestRenderHasHelperOnList(t *testing.T) {
	rule := map[string]interface{}{"tag": []interface{}{"critical", "web"}}
	out, err := tmpl.Render(`{{ if (has .tag "critical") }}Full{{ else }}Basic{{ end }}`, rule)
	require.NoError(t, err)
	require.Equal(t, "Full", out)
}

func TestRenderPlainStringUnchanged(t *testing.T) {
	out, err := tmpl.Render("deny", map[string]interface{}{})
	require.NoError(t, err)
	require.Equal(t, "deny", out)
}

func TestRenderStringHelpers(t *testing.T) {
	rule := map[string]interface{}{"name": "TEMP-rule", "tag": []interface{}{"a", "b"}}
	out, err := tmpl.Render(`{{ lower .name }}|{{ join .tag "," }}`, rule)
	require.NoError(t, err)
	require.Equal(t, "temp-rule|a,b", out)
}
