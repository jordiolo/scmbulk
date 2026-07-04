package selection_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"scmbulk/pkg/config"
	"scmbulk/pkg/selection"
)

func rule(name, action string, tags ...string) map[string]interface{} {
	t := make([]interface{}, len(tags))
	for i, x := range tags {
		t[i] = x
	}
	return map[string]interface{}{"name": name, "action": action, "tag": t}
}

func TestMatchByActionAndTag(t *testing.T) {
	f, err := selection.New(config.Selection{
		Match: config.Match{Action: "allow", Tag: "legacy"},
	})
	require.NoError(t, err)
	require.True(t, f.Matches(rule("r1", "allow", "legacy", "web")))
	require.False(t, f.Matches(rule("r2", "deny", "legacy")))
	require.False(t, f.Matches(rule("r3", "allow", "web")))
}

func TestMatchByNameRegex(t *testing.T) {
	f, err := selection.New(config.Selection{Match: config.Match{NameRegex: "^TEMP-"}})
	require.NoError(t, err)
	require.True(t, f.Matches(rule("TEMP-1", "allow")))
	require.False(t, f.Matches(rule("PROD-1", "allow")))
}

func TestNamesFileAndMatchCombineWithAND(t *testing.T) {
	path := filepath.Join(t.TempDir(), "names.csv")
	require.NoError(t, os.WriteFile(path, []byte("name\nr1\nr2\n"), 0o600))

	f, err := selection.New(config.Selection{
		NamesFile: path,
		Match:     config.Match{Action: "allow"},
	})
	require.NoError(t, err)
	require.True(t, f.Matches(rule("r1", "allow")))  // in list AND allow
	require.False(t, f.Matches(rule("r1", "deny")))  // in list but not allow
	require.False(t, f.Matches(rule("r3", "allow"))) // allow but not in list
}
