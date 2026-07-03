// Package config loads and validates the scmbulk YAML configuration.
package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SCM holds the tenant credentials and target folder.
type SCM struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	TSGID        string `yaml:"tsg_id"`
	Folder       string `yaml:"folder"`
}

// Match are the AND conditions applied to a live rule in mode B.
type Match struct {
	Action    string `yaml:"action"`
	Tag       string `yaml:"tag"`
	NameRegex string `yaml:"name_regex"`
}

// Selection describes which rules mode B targets.
type Selection struct {
	Position  string `yaml:"position"`
	NamesFile string `yaml:"names_file"`
	Match     Match  `yaml:"match"`
}

// Change describes the mode B mutations. Values may contain Go templates.
type Change struct {
	Set    map[string]string   `yaml:"set"`
	Add    map[string][]string `yaml:"add"`
	Remove map[string][]string `yaml:"remove"`
}

// Config is the full scmbulk configuration.
type Config struct {
	SCM          SCM       `yaml:"scm"`
	DebugEnabled bool      `yaml:"debugenabled"`
	DryRun       bool      `yaml:"dryrun"`
	ResultsFile  string    `yaml:"resultsfile"`
	StopFirstOne bool      `yaml:"stopfirstone"`
	StopEvery    int       `yaml:"stopevery"`
	StopOnError  bool      `yaml:"stoponerror"`
	Selection    Selection `yaml:"selection"`
	Change       Change    `yaml:"change"`
}

// Load reads and validates the config file at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	switch {
	case c.SCM.ClientID == "":
		return errors.New("scm.client_id is required")
	case c.SCM.ClientSecret == "":
		return errors.New("scm.client_secret is required")
	case c.SCM.TSGID == "":
		return errors.New("scm.tsg_id is required")
	case c.SCM.Folder == "":
		return errors.New("scm.folder is required")
	}
	return nil
}
