package cmd

import "testing"

func TestResolveRuleType(t *testing.T) {
	cases := []struct {
		name        string
		flagChanged bool
		flagVal     string
		cfgVal      string
		want        string
		wantErr     bool
	}{
		{"neither -> security", false, "security", "", "security", false},
		{"flag only", true, "decryption", "", "decryption", false},
		{"config only", false, "security", "decryption", "decryption", false},
		{"both equal", true, "decryption", "decryption", "decryption", false},
		{"conflict", true, "security", "decryption", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveRuleType(c.flagChanged, c.flagVal, c.cfgVal)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}
