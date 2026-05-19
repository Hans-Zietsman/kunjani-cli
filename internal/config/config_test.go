package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsWhenNoFileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yml")

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := c.EffectiveHost(); got != "https://play.kunjani.co" {
		t.Errorf("EffectiveHost = %q, want https://play.kunjani.co", got)
	}
	if c.Token != "" {
		t.Errorf("Token = %q, want empty", c.Token)
	}
	if c.Configured() {
		t.Error("Configured() = true, want false")
	}
}

func TestSavePersistsWithSecurePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yml")

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	c.Host = "http://localhost:3000"
	c.Token = "knj_test"
	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("file mode = %#o, want 0600", mode)
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("Stat dir: %v", err)
	}
	if mode := dirInfo.Mode().Perm(); mode != 0o700 {
		t.Errorf("dir mode = %#o, want 0700", mode)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.EffectiveHost() != "http://localhost:3000" {
		t.Errorf("reloaded host = %q", reloaded.EffectiveHost())
	}
	if reloaded.Token != "knj_test" {
		t.Errorf("reloaded token = %q", reloaded.Token)
	}
	if !reloaded.Configured() {
		t.Error("reloaded.Configured() = false")
	}
}
