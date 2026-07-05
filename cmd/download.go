package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"scmbulk/pkg/runner"
)

var (
	dlFolder   string
	dlPosition string
	dlOut      string
)

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download the folder's security rules to a CSV",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dlFolder != "" {
			loadedConfig.SCM.Folder = dlFolder
		}
		client, err := newClient()
		if err != nil {
			return err
		}
		out := dlOut
		if out == "" {
			folder := strings.ReplaceAll(loadedConfig.SCM.Folder, " ", "_")
			out = fmt.Sprintf("rules_%s_%s.csv", folder, time.Now().Format("20060102_150405"))
		}
		n, err := runner.Download(client, mustSecuritySchema(), dlPosition, out)
		if err != nil {
			return err
		}
		fmt.Printf("downloaded %d rules to %s\n", n, out)
		return nil
	},
}

func init() {
	downloadCmd.Flags().StringVar(&dlFolder, "folder", "", "override the config folder")
	downloadCmd.Flags().StringVar(&dlPosition, "position", "both", "pre | post | both")
	downloadCmd.Flags().StringVar(&dlOut, "out", "", "output CSV path (default rules_<folder>_<ts>.csv)")
}
