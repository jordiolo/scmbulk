package selection_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"scmbulk/pkg/config"
	"scmbulk/pkg/selection"
)

func rule(name, action string, users ...string) map[string]interface{} {
	u := make([]interface{}, len(users))
	for i, x := range users {
		u[i] = x
	}
	return map[string]interface{}{"name": name, "action": action, "source_user": u}
}

func TestScalarEquals(t *testing.T) {
	f, err := selection.New(config.Selection{Match: map[string]interface{}{"action": "allow"}})
	require.NoError(t, err)
	require.True(t, f.Matches(rule("r1", "allow")))
	require.False(t, f.Matches(rule("r2", "deny")))
}

func TestSingleValueOnListContains(t *testing.T) {
	f, err := selection.New(config.Selection{Match: map[string]interface{}{"source_user": "u1"}})
	require.NoError(t, err)
	require.True(t, f.Matches(rule("r1", "allow", "u1", "u2")))
	require.False(t, f.Matches(rule("r2", "allow", "u2")))
}

func TestBareListIsAny(t *testing.T) {
	f, err := selection.New(config.Selection{Match: map[string]interface{}{
		"source_user": []interface{}{"u1", "u9"},
	}})
	require.NoError(t, err)
	require.True(t, f.Matches(rule("r1", "allow", "u1")))       // has one
	require.False(t, f.Matches(rule("r2", "allow", "u2", "u3"))) // has none
}

func TestAllContainsAll(t *testing.T) {
	f, err := selection.New(config.Selection{Match: map[string]interface{}{
		"source_user": map[string]interface{}{"all": []interface{}{"u1", "u2", "u3"}},
	}})
	require.NoError(t, err)
	require.True(t, f.Matches(rule("r1", "allow", "u1", "u2", "u3", "u9"))) // all present
	require.False(t, f.Matches(rule("r2", "allow", "u1", "u2")))            // only 2 of 3
}

func TestAnyOperator(t *testing.T) {
	f, err := selection.New(config.Selection{Match: map[string]interface{}{
		"source_user": map[string]interface{}{"any": []interface{}{"u1", "u9"}},
	}})
	require.NoError(t, err)
	require.True(t, f.Matches(rule("r1", "allow", "u9")))
	require.False(t, f.Matches(rule("r2", "allow", "u2")))
}

func TestNameRegex(t *testing.T) {
	f, err := selection.New(config.Selection{Match: map[string]interface{}{"name_regex": "^TEMP-"}})
	require.NoError(t, err)
	require.True(t, f.Matches(rule("TEMP-1", "allow")))
	require.False(t, f.Matches(rule("PROD-1", "allow")))
}

func TestAbsentFieldNeverMatches(t *testing.T) {
	f, err := selection.New(config.Selection{Match: map[string]interface{}{"application": "web-browsing"}})
	require.NoError(t, err)
	require.False(t, f.Matches(rule("r1", "allow", "u1"))) // rule has no "application"
}

func TestMultipleKeysAnd(t *testing.T) {
	f, err := selection.New(config.Selection{Match: map[string]interface{}{
		"action":      "allow",
		"source_user": map[string]interface{}{"all": []interface{}{"u1", "u2"}},
	}})
	require.NoError(t, err)
	require.True(t, f.Matches(rule("r1", "allow", "u1", "u2")))
	require.False(t, f.Matches(rule("r2", "deny", "u1", "u2")))  // action fails
	require.False(t, f.Matches(rule("r3", "allow", "u1")))       // all fails
}

func TestTagBackwardCompat(t *testing.T) {
	r := map[string]interface{}{"name": "r1", "action": "allow", "tag": []interface{}{"legacy", "web"}}
	f, err := selection.New(config.Selection{Match: map[string]interface{}{"tag": "legacy"}})
	require.NoError(t, err)
	require.True(t, f.Matches(r))
}

func TestNamesFileIntersection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "names.csv")
	require.NoError(t, os.WriteFile(path, []byte("name\nr1\n"), 0o600))
	f, err := selection.New(config.Selection{
		NamesFile: path,
		Match:     map[string]interface{}{"action": "allow"},
	})
	require.NoError(t, err)
	require.True(t, f.Matches(rule("r1", "allow")))
	require.False(t, f.Matches(rule("r1", "deny")))  // in list, wrong action
	require.False(t, f.Matches(rule("r2", "allow"))) // right action, not in list
}

func TestInvalidMatchMapErrors(t *testing.T) {
	_, err := selection.New(config.Selection{Match: map[string]interface{}{
		"source_user": map[string]interface{}{"nope": []interface{}{"u1"}},
	}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "source_user")
}

func TestInvalidRegexErrors(t *testing.T) {
	_, err := selection.New(config.Selection{Match: map[string]interface{}{"name_regex": "("}})
	require.Error(t, err)
}
