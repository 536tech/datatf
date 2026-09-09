package cli

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/536tech/datatf/internal/contract"
	"github.com/536tech/datatf/internal/fakews"
	"github.com/536tech/datatf/internal/inventory"
)

func TestExportNamedCatalog(t *testing.T) {
	srv := fakews.New(t)
	srv.Fail("GET", "/api/2.1/unity-catalog/catalogs/shared_ref", http.StatusForbidden, "denied")
	isolateAuth(t, srv)
	out := filepath.Join(t.TempDir(), "out")
	code, stdout, stderr := run(t, "export", "--json", "--resources", "catalogs",
		"--name", "sales", "--out", out)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var report contract.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if report.Status != "complete" || report.Counts["catalogs"] != 1 ||
		report.Counts["schemas"] != 2 || report.Counts["workspace_bindings"] != 1 {
		t.Fatalf("unexpected selection: %+v", report)
	}
	assertNamedCatalogFiles(t, out)
}

func assertNamedCatalogFiles(t *testing.T, out string) {
	t.Helper()
	for _, file := range []string{"terraform.tfvars", "imports.tf", "export-report.json"} {
		data, err := os.ReadFile(filepath.Join(out, file))
		if err != nil || !strings.Contains(string(data), "sales") ||
			strings.Contains(string(data), "shared_ref") {
			t.Fatalf("unexpected %s: %s (%v)", file, data, err)
		}
	}
}

func TestNameRequiresOneGroup(t *testing.T) {
	for _, command := range []string{"export", "inventory"} {
		for _, resources := range [][]string{nil, {"--resources", "catalogs,warehouses"}} {
			args := append([]string{command, "--name", "sales"}, resources...)
			code, _, stderr := run(t, args...)
			if code != exitUsage || !strings.Contains(stderr, "exactly one resource group") {
				t.Fatalf("%v: exit %d: %s", args, code, stderr)
			}
		}
		code, _, stderr := run(t, command, "--resources", "catalogs", "--name", "")
		if code != exitUsage || !strings.Contains(stderr, "must not be empty") {
			t.Fatalf("%s: exit %d: %s", command, code, stderr)
		}
	}
}

func TestUnknownNameBlocksTerraform(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	out := filepath.Join(t.TempDir(), "out")
	code, stdout, stderr := run(t, "export", "--json", "--resources", "catalogs",
		"--name", "missing", "--out", out)
	var report contract.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("exit %d: %s; invalid JSON: %v", code, stderr, err)
	}
	if code != exitErr || report.Status != "partial" || len(report.Issues) != 1 {
		t.Fatalf("exit %d: %+v", code, report)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("failed selection wrote files: %v", err)
	}
}

func TestInventoryNamedObjectJSON(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	out := filepath.Join(t.TempDir(), "out")
	code, stdout, stderr := run(t, "inventory", "--json", "--resources", "warehouses",
		"--name", "Analytics WH", "--out", out)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var inv inventory.Inventory
	if err := json.Unmarshal([]byte(stdout), &inv); err != nil {
		t.Fatal(err)
	}
	if inv.Name == nil || *inv.Name != "Analytics WH" || len(inv.Warehouses) != 1 {
		t.Fatalf("unexpected inventory: %+v", inv)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("inventory --json wrote files: %v", err)
	}
}

func TestNamedExportKeepsPartialGuard(t *testing.T) {
	srv := fakews.New(t)
	srv.Fail("GET", "/api/2.0/permissions/warehouses/wh1", http.StatusForbidden, "denied")
	isolateAuth(t, srv)
	for _, allow := range []bool{false, true} {
		out := filepath.Join(t.TempDir(), "out")
		args := []string{"export", "--json", "--resources", "warehouses",
			"--name", "Analytics WH", "--out", out}
		if allow {
			args = append(args, "--allow-partial")
		}
		code, stdout, stderr := run(t, args...)
		var report contract.Report
		if err := json.Unmarshal([]byte(stdout), &report); err != nil {
			t.Fatalf("exit %d: %s; invalid JSON: %v", code, stderr, err)
		}
		if (code == exitOK) != allow || report.Status != "partial" || len(report.Issues) != 1 {
			t.Fatalf("allow=%t: exit %d: %+v", allow, code, report)
		}
		_, err := os.Stat(filepath.Join(out, "imports.tf"))
		if (err == nil) != allow {
			t.Fatalf("allow=%t: imports file: %v", allow, err)
		}
	}
}

func TestSelectionHelp(t *testing.T) {
	for _, command := range []string{"export", "inventory"} {
		code, stdout, stderr := run(t, command, "--help")
		if code != exitOK || !strings.Contains(stdout, "--resources catalogs --name sales") {
			t.Fatalf("%s help: exit %d: %s %s", command, code, stdout, stderr)
		}
	}
}
