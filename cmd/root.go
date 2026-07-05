// Package cmd wires the scmbulk CLI commands.
package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"scmbulk/pkg/config"
	"scmbulk/pkg/rules"
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

func mustSecuritySchema() *rules.Schema {
	s, err := rules.SchemaFor("security")
	if err != nil {
		panic(err)
	}
	return s
}
