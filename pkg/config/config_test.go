package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"scmbulk/pkg/config"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestLoadValidConfig(t *testing.T) {
	path := writeConfig(t, `
scm:
  client_id: "cid"
  client_secret: "secret"
  tsg_id: "123"
  folder: "Mobile Users"
stopevery: 25
change:
  set:
    action: "deny"
  add:
    tag: ["reviewed"]
`)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.Equal(t, "cid", cfg.SCM.ClientID)
	require.Equal(t, "Mobile Users", cfg.SCM.Folder)
	require.Equal(t, 25, cfg.StopEvery)
	require.Equal(t, "deny", cfg.Change.Set["action"])
	require.Equal(t, []string{"reviewed"}, cfg.Change.Add["tag"])
}

func TestLoadMissingClientIDFails(t *testing.T) {
	path := writeConfig(t, `
scm:
  client_secret: "secret"
  tsg_id: "123"
  folder: "Mobile Users"
`)
	_, err := config.Load(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "client_id")
}

func TestLoadRuleType(t *testing.T) {
	path := writeConfig(t, `
scm:
  client_id: "cid"
  client_secret: "secret"
  tsg_id: "123"
  folder: "Mobile Users"
rule_type: decryption
`)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.Equal(t, "decryption", cfg.RuleType)
}
