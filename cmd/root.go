// Package cmd wires the scmbulk CLI commands.
package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"scmbulk/pkg/config"
	"scmbulk/pkg/rules"
	"scmbulk/pkg/scm"
)

var (
	configPath   string
	loadedConfig *config.Config
	ruleTypeFlag string
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
	rootCmd.PersistentFlags().StringVar(&ruleTypeFlag, "type", "security", "rule type: security | decryption")
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(applyCmd)
}

// newClient builds an authenticated SCM client from the loaded config.
func newClient() (*scm.Client, error) {
	c := loadedConfig
	return scm.New(context.Background(), c.SCM.ClientID, c.SCM.ClientSecret, c.SCM.TSGID, c.SCM.Folder, c.DebugEnabled)
}

// resolveRuleType combines the --type flag and config rule_type. If both are
// set and differ, it errors; otherwise it returns whichever is set, defaulting
// to "security".
func resolveRuleType(flagChanged bool, flagVal, cfgVal string) (string, error) {
	switch {
	case flagChanged && cfgVal != "" && flagVal != cfgVal:
		return "", fmt.Errorf("conflicting rule type: --type %s vs config rule_type %s", flagVal, cfgVal)
	case flagChanged:
		return flagVal, nil
	case cfgVal != "":
		return cfgVal, nil
	default:
		return "security", nil
	}
}

// currentSchema resolves the effective rule type (flag + config) into a schema.
func currentSchema(cmd *cobra.Command) (*rules.Schema, error) {
	ruleType, err := resolveRuleType(cmd.Flags().Changed("type"), ruleTypeFlag, loadedConfig.RuleType)
	if err != nil {
		return nil, err
	}
	return rules.SchemaFor(ruleType)
}
