package cmd

import (
	"strings"
	"testing"
)

func TestNormalizeDiceOptionAcceptsValidValuesCaseInsensitive(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"loaded", "loaded"},
		{"Loaded", "loaded"},
		{"LOADED", "loaded"},
		{"random", "random"},
		{"Random", "random"},
		{"  loaded  ", "loaded"},
	}
	for _, c := range cases {
		got, err := normalizeDiceOption(c.in)
		if err != nil {
			t.Errorf("normalizeDiceOption(%q) returned error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("normalizeDiceOption(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeDiceOptionRejectsInvalidValues(t *testing.T) {
	for _, in := range []string{"", "crooked", "shuffle", "fixed", "lOaDeD ish"} {
		_, err := normalizeDiceOption(in)
		if err == nil {
			t.Errorf("normalizeDiceOption(%q) returned no error, expected one", in)
			continue
		}
		if !strings.Contains(err.Error(), "--dice-option") {
			t.Errorf("error for %q doesn't mention flag name: %v", in, err)
		}
	}
}

func TestCreateDeckFlagDefaultsToLoaded(t *testing.T) {
	cmd := newCreateDeckCmd("test")
	f := cmd.Flag("dice-option")
	if f == nil {
		t.Fatal("--dice-option flag missing on create-deck")
	}
	if f.DefValue != "loaded" {
		t.Errorf("--dice-option default = %q, want \"loaded\"", f.DefValue)
	}
}
