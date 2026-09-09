package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), args, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestVersion(t *testing.T) {
	code, out, _ := run(t, "version", "--plain")
	if code != exitOK {
		t.Fatalf("exit %d", code)
	}
	if strings.TrimSpace(out) != "dev" {
		t.Fatalf("unexpected version output %q", out)
	}
}

func TestRootWithoutCommandIsUsage(t *testing.T) {
	code, _, _ := run(t)
	if code != exitUsage {
		t.Fatalf("expected usage exit, got %d", code)
	}
}

func TestJSONAndPlainConflict(t *testing.T) {
	code, _, stderr := run(t, "version", "--json", "--plain")
	if code != exitUsage || !strings.Contains(stderr, "only one of") {
		t.Fatalf("expected usage error, got %d %q", code, stderr)
	}
}

func TestExportRejectsUnknownScope(t *testing.T) {
	code, _, stderr := run(t, "export", "--scope", "nope")
	if code != exitUsage || !strings.Contains(stderr, "--scope must be") {
		t.Fatalf("expected usage error, got %d %q", code, stderr)
	}
}

func TestLicenseFeaturesAreNotAvailable(t *testing.T) {
	tests := [][]string{
		{"license"},
		{"export", "--license", "license.json"},
	}
	for _, args := range tests {
		code, _, stderr := run(t, args...)
		if code == exitOK || !strings.Contains(stderr, "unknown") {
			t.Fatalf("expected unknown command or flag for %v, got %d %q", args, code, stderr)
		}
	}
}
