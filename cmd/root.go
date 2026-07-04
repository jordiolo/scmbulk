// Package cmd wires the scmbulk CLI commands.
package cmd

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"scmbulk/pkg/config"
	"scmbulk/pkg/scm"
)

var (
	configPath   string
	loadedConfig *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "scmbulk",
	Short: "Bulk change SCM security policies",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}
		loadedConfig = cfg
		return nil
	},
}

// applyCmd is a placeholder stub so the package compiles while Task 10 is in
// flight. Task 11 replaces this with the real "apply" command implementation.
// TODO(task-11): replace with the full apply command (cmd/apply.go).
var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply bulk changes to the folder's security rules (not yet implemented)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("apply is not yet implemented")
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "config.yaml", "path to config.yaml")
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(applyCmd)
}

// newClient builds an authenticated SCM client from the loaded config.
func newClient() (*scm.Client, error) {
	c := loadedConfig
	return scm.New(context.Background(), c.SCM.ClientID, c.SCM.ClientSecret, c.SCM.TSGID, c.SCM.Folder, c.DebugEnabled)
}
