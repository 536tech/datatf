package cli

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/536tech/datatf/internal/contract"
	"github.com/536tech/datatf/internal/fakews"
)

func TestExportResourceModules(t *testing.T) {
	for _, scope := range []string{"workspace", "shared"} {
		t.Run(scope, func(t *testing.T) {
			srv := fakews.New(t)
			isolateAuth(t, srv)
			out := filepath.Join(t.TempDir(), "out")
			code, _, stderr := run(t, "export", "--module-layout", "resources",
				"--scope", scope, "--out", out, "--scaffold")
			if code != exitOK {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			modules := readResourceModules(t, out)
			assertResourceImports(t, out, modules)
			want := 10
			if scope == "shared" {
				want = 5
			}
			if len(modules) != want {
				t.Fatalf("module coverage: got %v, want %d", modules, want)
			}
		})
	}
}

func readResourceModules(t *testing.T, out string) map[string]bool {
	t.Helper()
	file, diags := hclsyntax.ParseConfig(readGenerated(t, out, "main.tf"), "main.tf", hcl.InitialPos)
	if diags.HasErrors() {
		t.Fatal(diags)
	}
	modules := map[string]bool{}
	for _, block := range file.Body.(*hclsyntax.Body).Blocks {
		name := block.Labels[0]
		modules[name] = true
		registryName := strings.ReplaceAll(name, "_", "-")
		if name == "warehouse" {
			registryName = "sql-warehouse"
		}
		for key, want := range map[string]string{
			"source":  "registry.terraform.io/536tech/" + registryName + "/databricks",
			"version": "1.0.0",
		} {
			value, diags := block.Body.Attributes[key].Expr.Value(nil)
			if diags.HasErrors() || value.AsString() != want {
				t.Fatalf("%s %s = %v: %v", name, key, value, diags)
			}
		}
	}
	return modules
}

func assertResourceImports(t *testing.T, out string, modules map[string]bool) {
	t.Helper()
	var ex contract.Export
	if err := json.Unmarshal(readGenerated(t, out, "export.json"), &ex); err != nil {
		t.Fatal(err)
	}
	for _, imp := range ex.Imports {
		if !modules[imp.Module] {
			t.Fatalf("missing module for import: %+v", imp)
		}
	}
	if strings.Contains(string(readGenerated(t, out, "imports.tf")), "module.workspace.") {
		t.Fatal("resource imports must not include the workspace pattern")
	}
}

func TestResourceModuleSelection(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	out := filepath.Join(t.TempDir(), "out")
	args := []string{"export", "--module-layout", "resources", "--resources", "warehouses",
		"--name", "Analytics WH", "--out", out, "--scaffold", "--module-version", "1.0.0"}
	code, _, stderr := run(t, args...)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	main := string(readGenerated(t, out, "main.tf"))
	if strings.Count(main, `module "`) != 1 || !strings.Contains(main, `module "warehouse"`) {
		t.Fatalf("unexpected selected modules: %s", main)
	}
	vars := string(readGenerated(t, out, "terraform.tfvars"))
	if !strings.Contains(vars, `"Analytics WH"`) || !strings.Contains(vars, "warehouse =") {
		t.Fatalf("unexpected standalone inputs: %s", vars)
	}
	assertResourceOverwriteRefused(t, out, args)
}

func assertResourceOverwriteRefused(t *testing.T, out string, args []string) {
	t.Helper()
	before := readGenerated(t, out, "main.tf")
	code, _, stderr := run(t, args...)
	if code != exitErr || !strings.Contains(stderr, "refusing to overwrite") {
		t.Fatalf("overwrite exit %d: %s", code, stderr)
	}
	if string(readGenerated(t, out, "main.tf")) != string(before) {
		t.Fatal("existing root changed")
	}
}

func TestResourceModulePartial(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	srv.Fail("GET", "/api/2.0/permissions/warehouses/wh1", http.StatusForbidden, "denied")
	partial := filepath.Join(t.TempDir(), "partial")
	code, _, stderr := run(t, "export", "--module-layout", "resources", "--resources", "warehouses",
		"--scaffold", "--out", partial)
	if code != exitErr || !strings.Contains(stderr, "partial") {
		t.Fatalf("partial exit %d: %s", code, stderr)
	}
	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Fatal("partial export wrote files")
	}
}

func TestResourceModuleOptionConflicts(t *testing.T) {
	for _, options := range [][]string{
		{"--module-layout", "unknown"},
		{"--module-layout", "resources", "--root-module", "workspace"},
		{"--module-layout", "resources", "--module-source", "../module"},
		{"--module-layout", "resources", "--module-version", "~> 0.1"},
	} {
		code, _, stderr := run(t, append([]string{"export", "--scaffold"}, options...)...)
		if code != exitUsage || !strings.Contains(stderr, "--module-") {
			t.Fatalf("%v: exit %d: %s", options, code, stderr)
		}
	}
}

func readGenerated(t *testing.T, out, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(out, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
