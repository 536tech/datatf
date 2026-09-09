package telemetry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConsentDefaultsWithoutFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "datatf", "telemetry.json")
	getenv := func(string) string { return "" }
	if status := Resolve(path, getenv); status.Enabled || status.Source != "default" {
		t.Fatalf("default: %+v", status)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatal("status created a directory")
	}
}

func TestConsentSavedChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "datatf", "telemetry.json")
	getenv := func(string) string { return "" }
	for _, enabled := range []bool{true, false, true} {
		if err := SaveConsent(path, enabled); err != nil {
			t.Fatal(err)
		}
		if status := Resolve(path, getenv); status.Enabled != enabled || status.Source != "config" {
			t.Fatalf("saved consent: %+v", status)
		}
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 1 {
		t.Fatalf("unexpected configuration artifacts: %v %v", files, err)
	}
}

func TestConsentRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.json")
	if err := SaveConsent(path, true); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(path, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if status := Resolve(link, func(string) string { return "" }); status.Enabled {
		t.Fatal("symlink granted consent")
	}
}

func TestConsentPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry.json")
	if err := SaveConsent(path, true); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		env     map[string]string
		enabled bool
		source  string
	}{
		{map[string]string{"DATATF_TELEMETRY": "0"}, false, "DATATF_TELEMETRY"},
		{map[string]string{"DATATF_TELEMETRY": "invalid"}, false, "DATATF_TELEMETRY"},
		{map[string]string{"DO_NOT_TRACK": "1", "DATATF_TELEMETRY": "1"}, false, "DO_NOT_TRACK"},
		{map[string]string{"DO_NOT_TRACK": "true"}, false, "DO_NOT_TRACK"},
		{map[string]string{"CI": "true"}, false, "automation"},
		{map[string]string{"CI": "false"}, true, "config"},
		{map[string]string{"CODEX_THREAD_ID": "canary"}, false, "automation"},
		{map[string]string{"CLAUDECODE": "1"}, false, "automation"},
		{map[string]string{"CI": "1", "DATATF_TELEMETRY": "1"}, true, "DATATF_TELEMETRY"},
	}
	for _, test := range tests {
		status := Resolve(path, func(key string) string { return test.env[key] })
		if status.Enabled != test.enabled || status.Source != test.source {
			t.Errorf("environment %v: %+v", test.env, status)
		}
	}
}

func TestMalformedConsentFailsClosedWithoutEchoingContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telemetry.json")
	for _, data := range []string{
		`{"schema_version":1,"enabled":true`, `{"enabled":true}`, "null",
		`{"schema_version":2,"enabled":true}`, `{"schema_version":1,"enabled":"true"}`,
		`{"schema_version":1,"enabled":true,"canary":"secret"}`,
		`{"schema_version":1,"enabled":true} {"canary":1}`, strings.Repeat("canary", 1000),
	} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		status := Resolve(path, func(string) string { return "" })
		if status.Enabled || status.ConfigError == "" || strings.Contains(status.ConfigError, "canary") {
			t.Fatalf("malformed consent: %+v", status)
		}
	}
}

func TestConsentFilesystemFailures(t *testing.T) {
	dir := t.TempDir()
	if status := Resolve(dir, func(string) string { return "" }); status.Enabled {
		t.Fatal("a directory cannot grant consent")
	}
	path := filepath.Join(dir, "file")
	if err := os.WriteFile(path, []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := SaveConsent(filepath.Join(path, "telemetry.json"), true); err == nil {
		t.Fatal("expected a configuration write error")
	}
	if err := SaveConsent(dir, true); err == nil {
		t.Fatal("must not replace a directory")
	}
}
