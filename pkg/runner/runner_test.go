package runner_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"scmbulk/pkg/config"
	"scmbulk/pkg/rules"
	"scmbulk/pkg/runner"
)

// fakeClient implements runner.RuleClient in memory.
type fakeClient struct {
	list    map[string][]map[string]interface{} // position -> rules
	byID    map[string]map[string]interface{}
	updated map[string]map[string]interface{}
}

func newFake() *fakeClient {
	return &fakeClient{
		list:    map[string][]map[string]interface{}{},
		byID:    map[string]map[string]interface{}{},
		updated: map[string]map[string]interface{}{},
	}
}

func (f *fakeClient) ListSecurityRules(position string) ([]map[string]interface{}, error) {
	return f.list[position], nil
}
func (f *fakeClient) GetSecurityRule(id string) (map[string]interface{}, error) {
	src := f.byID[id]
	clone := map[string]interface{}{}
	for k, v := range src {
		clone[k] = v
	}
	return clone, nil
}
func (f *fakeClient) UpdateSecurityRule(id string, payload map[string]interface{}) error {
	f.updated[id] = payload
	return nil
}

func alwaysContinue(string) bool { return true }

func TestApplyCSVWritesOnlyChanged(t *testing.T) {
	f := newFake()
	f.byID["abc"] = map[string]interface{}{"id": "abc", "name": "r1", "action": "allow"}

	rows := []map[string]string{
		{"id": "abc", "name": "r1", "action": "deny"},
	}
	res, err := runner.ApplyCSV(f, rows, runner.Options{Confirm: alwaysContinue, Out: &bytes.Buffer{}})
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, "ok", res[0].Status)
	require.Contains(t, res[0].ChangedFields, "action")
	require.Equal(t, "deny", f.updated["abc"]["action"])
}

func TestApplyCSVDryRunDoesNotUpdate(t *testing.T) {
	f := newFake()
	f.byID["abc"] = map[string]interface{}{"id": "abc", "name": "r1", "action": "allow"}
	rows := []map[string]string{{"id": "abc", "action": "deny"}}

	res, err := runner.ApplyCSV(f, rows, runner.Options{DryRun: true, Confirm: alwaysContinue, Out: &bytes.Buffer{}})
	require.NoError(t, err)
	require.Equal(t, "dry-run", res[0].Status)
	require.Empty(t, f.updated)
}

func TestApplySelectSetAddRemoveWithTemplate(t *testing.T) {
	f := newFake()
	r := map[string]interface{}{"id": "1", "name": "r1", "action": "allow", "tag": []interface{}{"legacy"}}
	f.list["pre"] = []map[string]interface{}{r}
	f.byID["1"] = r

	sel := config.Selection{Position: "pre", Match: config.Match{Action: "allow"}}
	change := config.Change{
		Set:    map[string]string{"action": `{{ if (eq .action "allow") }}deny{{ else }}drop{{ end }}`},
		Add:    map[string][]string{"tag": {"reviewed"}},
		Remove: map[string][]string{"tag": {"legacy"}},
	}
	res, err := runner.ApplySelect(f, sel, change, runner.Options{Confirm: alwaysContinue, Out: &bytes.Buffer{}})
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, "ok", res[0].Status)
	require.Equal(t, "deny", f.updated["1"]["action"])
	require.ElementsMatch(t, []interface{}{"reviewed"}, f.updated["1"]["tag"])
}

func TestWriteResults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "res.csv")
	require.NoError(t, runner.WriteResults(path, []runner.Result{
		{ID: "1", Name: "r1", Position: "pre", Status: "ok", ChangedFields: "action", Message: ""},
	}))
	got, err := rules.ReadCSV(path)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "ok", got[0]["status"])
}

func TestPositionsExpandsBoth(t *testing.T) {
	require.Equal(t, []string{"pre", "post"}, runner.Positions("both"))
	require.Equal(t, []string{"pre"}, runner.Positions("pre"))
}
