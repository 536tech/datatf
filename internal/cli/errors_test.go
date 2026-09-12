package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/536tech/datatf/internal/fakews"
)

func decodeFailure(t *testing.T, stderr string) map[string]any {
	t.Helper()
	var envelope struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal([]byte(stderr), &envelope); err != nil {
		t.Fatalf("invalid diagnostic JSON: %v: %s", err, stderr)
	}
	if envelope.Error["hint"] == nil || envelope.Error["message"] == nil {
		t.Fatalf("missing recovery information: %s", stderr)
	}
	return envelope.Error
}

func TestUsageErrorsAreConsistent(t *testing.T) {
	tests := [][]string{
		{}, {"exprot"}, {"--doctor"}, {"export", "--no-such-flag"},
		{"export", "--scope"}, {"export", "--scope", "invalid"},
		{"auth", "no-such-command"}, {"auth", "status", "extra"},
		{"version", "extra"}, {"completion", "invalid"},
	}
	for _, args := range tests {
		code, out, stderr := run(t, append([]string{"--json"}, args...)...)
		if code != exitUsage || out != "" {
			t.Fatalf("%v: exit %d: stdout=%s stderr=%s", args, code, out, stderr)
		}
		failure := decodeFailure(t, stderr)
		if failure["code"] != "invalid_usage" || !strings.Contains(failure["hint"].(string), "--help") {
			t.Fatalf("%v: %v", args, failure)
		}
	}
}

func TestUnknownCommandSuggestsCorrection(t *testing.T) {
	code, _, stderr := run(t, "exprot")
	if code != exitUsage || !strings.Contains(stderr, "export") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}

func TestAuthenticationFailureDiagnostic(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	t.Setenv("DATABRICKS_TOKEN", "diagnostic-secret-token")
	srv.Fail("GET", "/api/2.0/preview/scim/v2/Me", http.StatusUnauthorized, "invalid credentials")
	code, out, stderr := run(t, "--json", "auth", "status")
	if code != exitErr || out != "" {
		t.Fatalf("exit %d: %s %s", code, out, stderr)
	}
	failure := decodeFailure(t, stderr)
	if failure["code"] != "authentication_failed" || failure["http_status"] != float64(401) {
		t.Fatalf("unexpected diagnostic: %v", failure)
	}
	if strings.Contains(stderr, "diagnostic-secret-token") {
		t.Fatal("diagnostic exposed the configured token")
	}
}

type brokenOutput struct{}

func (brokenOutput) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestBrokenJSONOutputFails(t *testing.T) {
	var stderr bytes.Buffer
	code := Execute(context.Background(), []string{"--json", "version"},
		strings.NewReader(""), brokenOutput{}, &stderr)
	if code != exitErr {
		t.Fatalf("broken output exited %d", code)
	}
	if failure := decodeFailure(t, stderr.String()); failure["code"] != "operation_failed" {
		t.Fatalf("unexpected diagnostic: %v", failure)
	}
}

func TestMissingProfileDiagnostic(t *testing.T) {
	isolateAuth(t, fakews.New(t))
	t.Setenv("DATABRICKS_TOKEN", "")
	configPath := os.Getenv("DATABRICKS_CONFIG_FILE")
	if err := os.WriteFile(configPath, []byte("[other]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := run(t, "--json", "auth", "status", "--profile", "missing-profile")
	if code != exitErr || out != "" {
		t.Fatalf("exit %d: %s %s", code, out, stderr)
	}
	failure := decodeFailure(t, stderr)
	if failure["code"] != "configuration_error" ||
		!strings.Contains(failure["hint"].(string), "databricks auth profiles") {
		t.Fatalf("unexpected diagnostic: %v", failure)
	}
}

func TestPartialInventoryFailsWithUsefulJSON(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	srv.Fail("GET", "/api/2.0/sql/warehouses", http.StatusForbidden, "denied")
	outDir := filepath.Join(t.TempDir(), "out")
	code, out, stderr := run(t, "--json", "inventory", "--resources", "warehouses", "--out", outDir)
	if code != exitErr {
		t.Fatalf("partial inventory exited %d: %s %s", code, out, stderr)
	}
	assertDeniedIssue(t, out)
	if failure := decodeFailure(t, stderr); failure["code"] != "partial_result" {
		t.Fatalf("unexpected diagnostic: %v", failure)
	}
	if _, err := os.Stat(outDir); !os.IsNotExist(err) {
		t.Fatalf("JSON inventory wrote files: %v", err)
	}
}

func TestPartialInventorySavesDiagnosticFiles(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	srv.Fail("GET", "/api/2.0/sql/warehouses", http.StatusForbidden, "denied")
	outDir := t.TempDir()
	code, _, stderr := run(t, "inventory", "--quiet", "--resources", "warehouses", "--out", outDir)
	if code != exitErr || !strings.Contains(stderr, "permissions.md") {
		t.Fatalf("text inventory: exit %d: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(outDir, "inventory-report.json")); err != nil {
		t.Fatalf("partial inventory lost its report: %v", err)
	}
}

func TestErrorMessagesCannotInjectTerminalControls(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	srv.Fail("GET", "/api/2.0/sql/warehouses", http.StatusForbidden, "denied\x1b[2J\nhint: run unsafe")
	code, _, stderr := run(t, "export", "--quiet", "--resources", "warehouses")
	if code != exitErr || strings.Contains(stderr, "\x1b") ||
		strings.Contains(stderr, "\nhint: run unsafe") {
		t.Fatalf("unsafe diagnostic: %d %q", code, stderr)
	}
}

func TestCanceledReadReturnsJSONError(t *testing.T) {
	isolateAuth(t, fakews.New(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	code := Execute(ctx, []string{"--json", "inventory"}, strings.NewReader(""), &stdout, &stderr)
	if code != exitErr || stdout.Len() != 0 {
		t.Fatalf("exit %d: %s %s", code, &stdout, &stderr)
	}
	if failure := decodeFailure(t, stderr.String()); failure["code"] != "canceled" {
		t.Fatalf("unexpected diagnostic: %v", failure)
	}
}

func assertDeniedIssue(t *testing.T, stdout string) {
	t.Helper()
	var result struct {
		Issues []map[string]any `json:"issues"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Issues) != 1 || result.Issues[0]["code"] != "permission_denied" ||
		result.Issues[0]["http_status"] != float64(403) || result.Issues[0]["hint"] == "" {
		t.Fatalf("unexpected issue: %s", stdout)
	}
}

func TestOutputFailureDiagnostic(t *testing.T) {
	isolateAuth(t, fakews.New(t))
	outDir := t.TempDir()
	existing := filepath.Join(outDir, "terraform.tfvars")
	if err := os.WriteFile(existing, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := run(t, "--json", "export", "--resources", "warehouses", "--out", outDir)
	if code != exitErr || out != "" {
		t.Fatalf("exit %d: %s %s", code, out, stderr)
	}
	failure := decodeFailure(t, stderr)
	if failure["code"] != "output_error" || !strings.Contains(failure["hint"].(string), "--out") {
		t.Fatalf("unexpected diagnostic: %v", failure)
	}
}

func TestTerminalTextNeutralizesControlAndFormatCharacters(t *testing.T) {
	for _, hostile := range []string{
		"a\x1b[2Kb", "a\rb", "a\u202eb", "a\u2066b", "a\u2028b", "a\u200fb",
	} {
		safe := terminalText(hostile)
		if strings.ContainsAny(safe, "\x1b\r\u202e\u2066\u2028\u200f") {
			t.Fatalf("%q was not neutralized: %q", hostile, safe)
		}
		if !strings.HasPrefix(safe, "a") || !strings.HasSuffix(safe, "b") {
			t.Fatalf("%q lost visible text: %q", hostile, safe)
		}
	}
	if terminalText("Analytics WH") != "Analytics WH" {
		t.Fatal("plain text changed")
	}
}

func TestProgressOutputNeutralizesWorkspaceNames(t *testing.T) {
	var stderr strings.Builder
	rc := &runtime{stderr: &stderr, g: &globals{}}
	rc.progress()("  Warehouse: %s", "prod\x1b[2K\rok")
	if got := stderr.String(); strings.Contains(got, "\x1b") || strings.Contains(got, "\r") {
		t.Fatalf("progress leaked control characters: %q", got)
	}
}
