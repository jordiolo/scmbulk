package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"scmbulk/pkg/rules"
	"scmbulk/pkg/runner"
)

var (
	applyFile   string
	applySelect bool
	applyDryRun bool
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply bulk changes (mode A: --file, mode B: --select)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if (applyFile == "") == (!applySelect) {
			return errors.New("choose exactly one mode: --file <csv> or --select")
		}
		schema, err := currentSchema(cmd)
		if err != nil {
			return err
		}
		client, err := newClient()
		if err != nil {
			return err
		}
		opts := runner.Options{
			DryRun:       applyDryRun || loadedConfig.DryRun,
			StopFirstOne: loadedConfig.StopFirstOne,
			StopEvery:    loadedConfig.StopEvery,
			StopOnError:  loadedConfig.StopOnError,
			Confirm:      confirmStdin,
			ConfirmError: confirmErrorStdin,
			Out:          os.Stdout,
		}

		var results []runner.Result
		if applyFile != "" {
			rows, err := rules.ReadCSV(applyFile)
			if err != nil {
				return err
			}
			results, err = runner.ApplyCSV(client, schema, rows, opts)
			if err != nil {
				return err
			}
		} else {
			results, err = runner.ApplySelect(client, schema, loadedConfig.Selection, loadedConfig.Change, opts)
			if err != nil {
				return err
			}
		}

		out := loadedConfig.ResultsFile
		if out == "" {
			out = fmt.Sprintf("results_%s.csv", time.Now().Format("20060102_150405"))
		}
		if err := runner.WriteResults(out, results); err != nil {
			return err
		}
		fmt.Printf("processed %d rules; results written to %s\n", len(results), out)
		return nil
	},
}

// confirmStdin asks the user y/n on stdin; empty/"y" continues. If stdin is
// closed or unreadable before any input arrives (e.g. a non-interactive run),
// it declines rather than assuming consent.
func confirmStdin(prompt string) bool {
	fmt.Printf("%s [Y/n] ", prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "" || line == "y" || line == "yes"
}

// confirmErrorStdin asks retry/continue/abort on stdin after a rule fails.
// Empty line or "c" continues past this rule, "r" retries the same
// operation, "a" aborts the whole run. Closed/unreadable stdin aborts,
// matching confirmStdin's fail-safe of not assuming consent.
func confirmErrorStdin(prompt string) runner.ErrorAction {
	fmt.Printf("%s [r]etry/[C]ontinue/[a]bort ", prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return runner.ActionAbort
	}
	switch strings.TrimSpace(strings.ToLower(line)) {
	case "r", "retry":
		return runner.ActionRetry
	case "a", "abort":
		return runner.ActionAbort
	default:
		return runner.ActionContinue
	}
}

func init() {
	applyCmd.Flags().StringVar(&applyFile, "file", "", "mode A: edited CSV to apply")
	applyCmd.Flags().BoolVar(&applySelect, "select", false, "mode B: use config selection + change")
	applyCmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "preview changes without writing")
}
