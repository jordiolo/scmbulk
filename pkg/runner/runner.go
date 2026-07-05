// Package runner orchestrates download and apply against a RuleClient.
package runner

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"scmbulk/pkg/config"
	"scmbulk/pkg/rules"
	tmpl "scmbulk/pkg/template"
)

// RuleClient is the subset of the SCM client the runner needs (fakeable).
type RuleClient interface {
	ListRules(resourcePath, position string) ([]map[string]interface{}, error)
	GetRule(resourcePath, id string) (map[string]interface{}, error)
	UpdateRule(resourcePath, id string, payload map[string]interface{}) error
}

// Result is one row of the results CSV.
type Result struct {
	ID            string
	Name          string
	Position      string
	Status        string // ok | error | skipped | dry-run
	ChangedFields string
	Message       string
}

// Options controls apply behaviour.
type Options struct {
	DryRun       bool
	StopFirstOne bool
	StopEvery    int
	StopOnError  bool
	Confirm      func(prompt string) bool // return false to abort; nil = always continue
	Out          io.Writer                // preview/log output; nil = os.Stdout
}

func (o Options) out() io.Writer {
	if o.Out != nil {
		return o.Out
	}
	return os.Stdout
}

func (o Options) confirm(prompt string) bool {
	if o.Confirm == nil {
		return true
	}
	return o.Confirm(prompt)
}

// Positions expands "both" into pre and post; otherwise returns [position].
func Positions(position string) []string {
	if position == "both" {
		return []string{"pre", "post"}
	}
	return []string{position}
}

// Download lists rules for the position(s) and writes them to outPath.
func Download(client RuleClient, schema *rules.Schema, position, outPath string) (int, error) {
	var rows []map[string]string
	for _, pos := range Positions(position) {
		list, err := client.ListRules(schema.ResourcePath, pos)
		if err != nil {
			return 0, err
		}
		for _, obj := range list {
			row := schema.ToRow(obj)
			row["position"] = pos
			rows = append(rows, row)
		}
	}
	if err := schema.WriteCSV(outPath, rows); err != nil {
		return 0, err
	}
	return len(rows), nil
}

// ApplyCSV applies an edited CSV (mode A) via per-row auto-diff.
func ApplyCSV(client RuleClient, schema *rules.Schema, rows []map[string]string, opts Options) ([]Result, error) {
	var results []Result
	processed := 0
	for _, row := range rows {
		id := row["id"]
		if id == "" {
			results = append(results, Result{Name: row["name"], Status: "error", Message: "missing id"})
			if opts.StopOnError && !opts.confirm("error occurred; continue?") {
				break
			}
			continue
		}
		live, err := client.GetRule(schema.ResourcePath, id)
		if err != nil {
			results = append(results, Result{ID: id, Name: row["name"], Status: "error", Message: err.Error()})
			if opts.StopOnError && !opts.confirm("error occurred; continue?") {
				break
			}
			continue
		}
		changes := schema.ApplyRow(live, row)
		res, stop := commit(client, schema, id, row["name"], row["position"], live, changes, &processed, opts)
		results = append(results, res)
		if stop {
			break
		}
	}
	return results, nil
}

// ApplySelect applies a declarative filter + change (mode B).
func ApplySelect(client RuleClient, schema *rules.Schema, sel config.Selection, change config.Change, opts Options) ([]Result, error) {
	filter, err := newFilter(sel)
	if err != nil {
		return nil, err
	}
	var results []Result
	processed := 0
	for _, pos := range Positions(sel.Position) {
		list, err := client.ListRules(schema.ResourcePath, pos)
		if err != nil {
			return nil, err
		}
		for _, summary := range list {
			if !filter.Matches(summary) {
				continue
			}
			id, _ := summary["id"].(string)
			name, _ := summary["name"].(string)
			live, err := client.GetRule(schema.ResourcePath, id)
			if err != nil {
				results = append(results, Result{ID: id, Name: name, Position: pos, Status: "error", Message: err.Error()})
				if opts.StopOnError && !opts.confirm("error occurred; continue?") {
					return results, nil
				}
				continue
			}
			changes, err := applyChange(schema, live, change)
			if err != nil {
				results = append(results, Result{ID: id, Name: name, Position: pos, Status: "error", Message: err.Error()})
				if opts.StopOnError && !opts.confirm("error occurred; continue?") {
					return results, nil
				}
				continue
			}
			res, stop := commit(client, schema, id, name, pos, live, changes, &processed, opts)
			results = append(results, res)
			if stop {
				return results, nil
			}
		}
	}
	return results, nil
}

func applyChange(schema *rules.Schema, live map[string]interface{}, change config.Change) ([]rules.FieldChange, error) {
	var changes []rules.FieldChange
	for field, valueTmpl := range change.Set {
		value, err := tmpl.Render(valueTmpl, live)
		if err != nil {
			return nil, err
		}
		if ch := schema.Set(live, field, value); ch != nil {
			changes = append(changes, *ch)
		}
	}
	for field, valueTmpls := range change.Add {
		if !schema.IsListField(field) {
			return nil, fmt.Errorf("field %q is not a list field; add/remove only apply to list fields", field)
		}
		values, err := renderAll(valueTmpls, live)
		if err != nil {
			return nil, err
		}
		if ch := schema.Add(live, field, values); ch != nil {
			changes = append(changes, *ch)
		}
	}
	for field, valueTmpls := range change.Remove {
		if !schema.IsListField(field) {
			return nil, fmt.Errorf("field %q is not a list field; add/remove only apply to list fields", field)
		}
		values, err := renderAll(valueTmpls, live)
		if err != nil {
			return nil, err
		}
		if ch := schema.Remove(live, field, values); ch != nil {
			changes = append(changes, *ch)
		}
	}
	return changes, nil
}

func renderAll(tmpls []string, live map[string]interface{}) ([]string, error) {
	out := make([]string, 0, len(tmpls))
	for _, s := range tmpls {
		v, err := tmpl.Render(s, live)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func commit(client RuleClient, schema *rules.Schema, id, name, position string, live map[string]interface{},
	changes []rules.FieldChange, processed *int, opts Options) (Result, bool) {

	res := Result{ID: id, Name: name, Position: position}
	if len(changes) == 0 {
		res.Status = "skipped"
		res.Message = "no changes"
		fmt.Fprintf(opts.out(), "- %s (%s): no changes\n", name, position)
		return res, false
	}

	res.ChangedFields = fieldNames(changes)
	printPreview(opts.out(), name, position, changes)

	if opts.DryRun {
		res.Status = "dry-run"
		return res, false
	}

	if err := client.UpdateRule(schema.ResourcePath, id, live); err != nil {
		res.Status = "error"
		res.Message = err.Error()
		if opts.StopOnError && !opts.confirm("error occurred; continue?") {
			return res, true
		}
		return res, false
	}
	res.Status = "ok"

	*processed++
	if opts.StopFirstOne && *processed == 1 {
		if !opts.confirm("first rule applied; continue?") {
			return res, true
		}
	}
	if opts.StopEvery > 0 && *processed%opts.StopEvery == 0 {
		if !opts.confirm(fmt.Sprintf("%d rules applied; continue?", *processed)) {
			return res, true
		}
	}
	return res, false
}

func printPreview(w io.Writer, name, position string, changes []rules.FieldChange) {
	fmt.Fprintf(w, "* %s (%s):\n", name, position)
	for _, c := range changes {
		fmt.Fprintf(w, "    %s: %q -> %q\n", c.Field, c.Old, c.New)
	}
}

func fieldNames(changes []rules.FieldChange) string {
	names := make([]string, len(changes))
	for i, c := range changes {
		names[i] = c.Field
	}
	return strings.Join(names, ";")
}

// resultColumns is the header for the results CSV.
var resultColumns = []string{"id", "name", "position", "status", "changed_fields", "message"}

// WriteResults writes the results CSV.
func WriteResults(path string, results []Result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(resultColumns); err != nil {
		return err
	}
	for _, r := range results {
		if err := w.Write([]string{r.ID, r.Name, r.Position, r.Status, r.ChangedFields, r.Message}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
