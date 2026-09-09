package cli

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/536tech/datatf/internal/contract"
	"github.com/536tech/datatf/internal/fakews"
)

func TestExportSelectedResources(t *testing.T) {
	srv := fakews.New(t)
	srv.Fail("GET", "/api/2.1/unity-catalog/catalogs", http.StatusForbidden, "denied")
	isolateAuth(t, srv)
	code, stdout, stderr := run(t, "export", "--json", "--resources", "warehouses",
		"--out", filepath.Join(t.TempDir(), "out"))
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var report contract.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if report.Imports != 2 || report.Counts["catalogs"] != 0 {
		t.Fatalf("unexpected selection: %+v", report)
	}
}

func TestSharedExportDoesNotReadWorkspaceResources(t *testing.T) {
	srv := fakews.New(t)
	srv.Fail("GET", "/api/2.0/sql/warehouses", http.StatusForbidden, "denied")
	isolateAuth(t, srv)
	code, _, stderr := run(t, "export", "--scope", "shared",
		"--out", filepath.Join(t.TempDir(), "out"))
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}

func TestExportPartialJSON(t *testing.T) {
	srv := fakews.New(t)
	srv.Fail("GET", "/api/2.0/sql/warehouses", http.StatusForbidden, "denied")
	isolateAuth(t, srv)
	out := filepath.Join(t.TempDir(), "out")
	code, stdout, stderr := run(t, "export", "--json", "--resources", "warehouses",
		"--out", out)
	var report contract.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("exit %d: %s; invalid JSON: %v", code, stderr, err)
	}
	if code != exitErr || report.Status != "partial" || len(report.Issues) != 1 {
		t.Fatalf("exit %d: %+v", code, report)
	}
	assertDeniedIssue(t, stdout)
	if failure := decodeFailure(t, stderr); failure["code"] != "partial_result" {
		t.Fatalf("unexpected failure: %+v", failure)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("partial export must not write output: %v", err)
	}
}
