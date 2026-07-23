package runner_test

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"scmbulk/pkg/config"
	"scmbulk/pkg/rules"
	"scmbulk/pkg/runner"
)

// fakeClient implements runner.RuleClient in memory.
type fakeClient struct {
	list            map[string][]map[string]interface{} // position -> rules
	byID            map[string]map[string]interface{}
	updated         map[string]map[string]interface{}
	failUpdatesLeft int // UpdateRule returns an error this many times before succeeding
}

func newFake() *fakeClient {
	return &fakeClient{
		list:    map[string][]map[string]interface{}{},
		byID:    map[string]map[string]interface{}{},
		updated: map[string]map[string]interface{}{},
	}
}

func (f *fakeClient) ListRules(_, position string) ([]map[string]interface{}, error) {
	return f.list[position], nil
}
func (f *fakeClient) GetRule(_, id string) (map[string]interface{}, error) {
	src := f.byID[id]
	clone := map[string]interface{}{}
	for k, v := range src {
		clone[k] = v
	}
	return clone, nil
}
func (f *fakeClient) UpdateRule(_, id string, payload map[string]interface{}) error {
	if f.failUpdatesLeft > 0 {
		f.failUpdatesLeft--
		return fmt.Errorf("simulated transient error")
	}
	f.updated[id] = payload
	return nil
}

func securitySchema(t *testing.T) *rules.Schema {
	t.Helper()
	s, err := rules.SchemaFor("security")
	require.NoError(t, err)
	return s
}

func alwaysContinue(string) bool { return true }

func changedFieldNames(changes []rules.FieldChange) []string {
	names := make([]string, len(changes))
	for i, c := range changes {
		names[i] = c.Field
	}
	return names
}

func TestApplyCSVWritesOnlyChanged(t *testing.T) {
	f := newFake()
	f.byID["abc"] = map[string]interface{}{"id": "abc", "name": "r1", "action": "allow"}

	rows := []map[string]string{
		{"id": "abc", "name": "r1", "action": "deny"},
	}
	res, err := runner.ApplyCSV(f, securitySchema(t), rows, runner.Options{Confirm: alwaysContinue, Out: &bytes.Buffer{}})
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, "ok", res[0].Status)
	require.Contains(t, changedFieldNames(res[0].Changes), "action")
	require.Equal(t, "deny", f.updated["abc"]["action"])
}

func TestApplyCSVDryRunDoesNotUpdate(t *testing.T) {
	f := newFake()
	f.byID["abc"] = map[string]interface{}{"id": "abc", "name": "r1", "action": "allow"}
	rows := []map[string]string{{"id": "abc", "action": "deny"}}

	res, err := runner.ApplyCSV(f, securitySchema(t), rows, runner.Options{DryRun: true, Confirm: alwaysContinue, Out: &bytes.Buffer{}})
	require.NoError(t, err)
	require.Equal(t, "dry-run", res[0].Status)
	require.Empty(t, f.updated)
}

func TestApplySelectSetAddRemoveWithTemplate(t *testing.T) {
	f := newFake()
	r := map[string]interface{}{"id": "1", "name": "r1", "action": "allow", "tag": []interface{}{"legacy"}}
	f.list["pre"] = []map[string]interface{}{r}
	f.byID["1"] = r

	sel := config.Selection{Position: "pre", Match: map[string]interface{}{"action": "allow"}}
	change := config.Change{
		Set:    map[string]string{"action": `{{ if (eq .action "allow") }}deny{{ else }}drop{{ end }}`},
		Add:    map[string][]string{"tag": {"reviewed"}},
		Remove: map[string][]string{"tag": {"legacy"}},
	}
	res, err := runner.ApplySelect(f, securitySchema(t), sel, change, runner.Options{Confirm: alwaysContinue, Out: &bytes.Buffer{}})
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, "ok", res[0].Status)
	require.Equal(t, "deny", f.updated["1"]["action"])
	require.ElementsMatch(t, []interface{}{"reviewed"}, f.updated["1"]["tag"])
	// add and remove both touch "tag": the reported change must be the single
	// net diff (legacy -> reviewed), not one entry per operation.
	require.Equal(t, []string{"action", "tag"}, changedFieldNames(res[0].Changes))
}

func TestApplySelectAddThenRemoveSameFieldReportsSingleNetChange(t *testing.T) {
	f := newFake()
	r := map[string]interface{}{"id": "1", "name": "r1", "action": "allow", "source_hip": []interface{}{"any"}}
	f.list["pre"] = []map[string]interface{}{r}
	f.byID["1"] = r

	sel := config.Selection{Position: "pre", Match: map[string]interface{}{"action": "allow"}}
	change := config.Change{
		Add:    map[string][]string{"source_hip": {"hip-a", "hip-b"}},
		Remove: map[string][]string{"source_hip": {"any"}},
	}
	res, err := runner.ApplySelect(f, securitySchema(t), sel, change, runner.Options{Confirm: alwaysContinue, Out: &bytes.Buffer{}})
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, "ok", res[0].Status)
	require.Equal(t, []string{"source_hip"}, changedFieldNames(res[0].Changes))
	require.ElementsMatch(t, []interface{}{"hip-a", "hip-b"}, f.updated["1"]["source_hip"])
}

func TestApplySelectNetNoOpChangeSkipsRule(t *testing.T) {
	// Adding and then removing the exact same value nets to no change at all.
	f := newFake()
	r := map[string]interface{}{"id": "1", "name": "r1", "action": "allow", "tag": []interface{}{"legacy"}}
	f.list["pre"] = []map[string]interface{}{r}
	f.byID["1"] = r

	sel := config.Selection{Position: "pre", Match: map[string]interface{}{"action": "allow"}}
	change := config.Change{
		Add:    map[string][]string{"tag": {"temp"}},
		Remove: map[string][]string{"tag": {"temp"}},
	}
	res, err := runner.ApplySelect(f, securitySchema(t), sel, change, runner.Options{Confirm: alwaysContinue, Out: &bytes.Buffer{}})
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, "skipped", res[0].Status)
	require.Empty(t, f.updated)
}

func TestApplySelectAddNonListFieldErrors(t *testing.T) {
	f := newFake()
	r := map[string]interface{}{"id": "1", "name": "r1", "action": "allow"}
	f.list["pre"] = []map[string]interface{}{r}
	f.byID["1"] = r

	sel := config.Selection{Position: "pre", Match: map[string]interface{}{"action": "allow"}}
	// add/remove only apply to list fields; "action" is a scalar -> must error.
	change := config.Change{Add: map[string][]string{"action": {"deny"}}}
	res, err := runner.ApplySelect(f, securitySchema(t), sel, change, runner.Options{Confirm: alwaysContinue, Out: &bytes.Buffer{}})
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, "error", res[0].Status)
	require.Contains(t, res[0].Message, "not a list field")
	require.Empty(t, f.updated)
}

func TestApplyCSVMissingIDErrors(t *testing.T) {
	f := newFake()
	rows := []map[string]string{{"name": "r1", "action": "deny"}}
	res, err := runner.ApplyCSV(f, securitySchema(t), rows, runner.Options{Confirm: alwaysContinue, Out: &bytes.Buffer{}})
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, "error", res[0].Status)
	require.Contains(t, res[0].Message, "missing id")
	require.Empty(t, f.updated)
}

func TestApplyCSVMissingIDPreservesPosition(t *testing.T) {
	f := newFake()
	rows := []map[string]string{{"name": "r1", "position": "pre", "action": "deny"}}
	res, err := runner.ApplyCSV(f, securitySchema(t), rows, runner.Options{Confirm: alwaysContinue, Out: &bytes.Buffer{}})
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, "error", res[0].Status)
	require.Contains(t, res[0].Message, "missing id")
	require.Equal(t, "pre", res[0].Position)
	require.Empty(t, f.updated)
}

func TestWriteResults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "res.csv")
	require.NoError(t, runner.WriteResults(path, []runner.Result{
		{
			ID: "1", Name: "r1", Position: "pre", Status: "ok",
			Changes: []rules.FieldChange{{Field: "action", Old: "allow", New: "deny"}},
		},
		{
			// A rule with more than one changed field must produce one row per
			// field, not a single row that mixes both diffs into one cell.
			ID: "2", Name: "r2", Position: "post", Status: "ok",
			Changes: []rules.FieldChange{
				{Field: "action", Old: "allow", New: "deny"},
				{Field: "tag", Old: "legacy", New: "reviewed"},
			},
		},
		{ID: "3", Name: "r3", Position: "pre", Status: "skipped", Message: "no changes"},
	}))

	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	require.NoError(t, err)

	require.Equal(t, []string{"id", "name", "position", "status", "field", "old_value", "new_value", "message"}, rows[0])
	require.Equal(t, []string{"1", "r1", "pre", "ok", "action", "allow", "deny", ""}, rows[1])
	require.Equal(t, []string{"2", "r2", "post", "ok", "action", "allow", "deny", ""}, rows[2])
	require.Equal(t, []string{"2", "r2", "post", "ok", "tag", "legacy", "reviewed", ""}, rows[3])
	require.Equal(t, []string{"3", "r3", "pre", "skipped", "", "", "", "no changes"}, rows[4])
}

func TestPositionsExpandsBoth(t *testing.T) {
	require.Equal(t, []string{"pre", "post"}, runner.Positions("both"))
	require.Equal(t, []string{"pre"}, runner.Positions("pre"))
}

func TestDownloadUsesSchemaResourcePath(t *testing.T) {
	f := newFake()
	f.list["pre"] = []map[string]interface{}{{"id": "d1", "name": "dec1", "action": "no-decrypt"}}
	dec, err := rules.SchemaFor("decryption")
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "out.csv")
	n, err := runner.Download(f, dec, "pre", path)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	got, err := rules.ReadCSV(path)
	require.NoError(t, err)
	require.Equal(t, "no-decrypt", got[0]["action"])
}

func TestApplyCSVProfileSettingInvalidValueErrors(t *testing.T) {
	// Mode A with unrecognized profile_setting cell must yield an error Result
	// and no UpdateRule call.
	f := newFake()
	f.byID["abc"] = map[string]interface{}{
		"id":              "abc",
		"name":            "r1",
		"profile_setting": map[string]interface{}{"group": []interface{}{"Best-Practice"}},
	}
	rows := []map[string]string{
		{"id": "abc", "name": "r1", "position": "pre", "profile_setting": "best-practice"},
	}
	res, err := runner.ApplyCSV(f, securitySchema(t), rows, runner.Options{Confirm: alwaysContinue, Out: &bytes.Buffer{}})
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, "error", res[0].Status)
	require.Contains(t, res[0].Message, "profile_setting")
	require.Empty(t, f.updated, "UpdateRule must not be called on codec error")
}

func TestApplyCSVRetryRecoversFromTransientError(t *testing.T) {
	f := newFake()
	f.byID["abc"] = map[string]interface{}{"id": "abc", "name": "r1", "action": "allow"}
	f.failUpdatesLeft = 1 // fails once, then succeeds

	rows := []map[string]string{{"id": "abc", "name": "r1", "action": "deny"}}

	retries := 0
	confirmError := func(string) runner.ErrorAction {
		retries++
		return runner.ActionRetry
	}

	res, err := runner.ApplyCSV(f, securitySchema(t), rows, runner.Options{
		StopOnError:  true,
		Confirm:      alwaysContinue,
		ConfirmError: confirmError,
		Out:          &bytes.Buffer{},
	})
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, "ok", res[0].Status)
	require.Equal(t, 1, retries)
	require.Equal(t, "deny", f.updated["abc"]["action"])
}

func TestApplyCSVErrorActionAbortStopsProcessing(t *testing.T) {
	f := newFake()
	f.byID["abc"] = map[string]interface{}{"id": "abc", "name": "r1", "action": "allow"}
	f.byID["def"] = map[string]interface{}{"id": "def", "name": "r2", "action": "allow"}
	f.failUpdatesLeft = 999 // every UpdateRule call fails

	rows := []map[string]string{
		{"id": "abc", "name": "r1", "action": "deny"},
		{"id": "def", "name": "r2", "action": "deny"},
	}

	res, err := runner.ApplyCSV(f, securitySchema(t), rows, runner.Options{
		StopOnError:  true,
		Confirm:      alwaysContinue,
		ConfirmError: func(string) runner.ErrorAction { return runner.ActionAbort },
		Out:          &bytes.Buffer{},
	})
	require.NoError(t, err)
	require.Len(t, res, 1, "must stop after the first failure")
	require.Equal(t, "error", res[0].Status)
}

func TestApplyCSVErrorActionContinueSkipsRule(t *testing.T) {
	f := newFake()
	f.byID["abc"] = map[string]interface{}{"id": "abc", "name": "r1", "action": "allow"}
	f.byID["def"] = map[string]interface{}{"id": "def", "name": "r2", "action": "allow"}
	f.failUpdatesLeft = 1 // only the first UpdateRule call fails

	rows := []map[string]string{
		{"id": "abc", "name": "r1", "action": "deny"},
		{"id": "def", "name": "r2", "action": "deny"},
	}

	res, err := runner.ApplyCSV(f, securitySchema(t), rows, runner.Options{
		StopOnError:  true,
		Confirm:      alwaysContinue,
		ConfirmError: func(string) runner.ErrorAction { return runner.ActionContinue },
		Out:          &bytes.Buffer{},
	})
	require.NoError(t, err)
	require.Len(t, res, 2)
	require.Equal(t, "error", res[0].Status)
	require.Equal(t, "ok", res[1].Status)
}
