package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(path, []byte("# comment\n\nDOTENV_A=one\r\nexport DOTENV_B=\"two words\"\nDOTENV_C=from-file\nDOTENV_EMPTY=\nnot a pair\n"), 0o600)
	t.Setenv("DOTENV_C", "from-env")
	for _, k := range []string{"DOTENV_A", "DOTENV_B", "DOTENV_EMPTY"} {
		os.Unsetenv(k)
		t.Cleanup(func() { os.Unsetenv(k) })
	}
	if err := LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{"DOTENV_A": "one", "DOTENV_B": "two words", "DOTENV_C": "from-env", "DOTENV_EMPTY": ""} {
		if got := os.Getenv(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if err := LoadDotEnv(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Errorf("missing file: %v", err)
	}
}
