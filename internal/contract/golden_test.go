package contract_test

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/536tech/datatf/internal/contract"
	"github.com/536tech/datatf/internal/emit"
	"github.com/536tech/datatf/internal/fakews"
	"github.com/536tech/datatf/internal/inventory"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestGolden(t *testing.T) {
	srv := fakews.New(t)
	inv, err := inventory.New(srv.Client(t)).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []contract.Scope{contract.ScopeWorkspace, contract.ScopeShared} {
		t.Run(string(scope), func(t *testing.T) {
			ex, issues := contract.Build(inv, scope)
			if len(issues) != 0 {
				t.Fatalf("build issues: %+v", issues)
			}
			rep := contract.NewReport(inv, ex, issues, "datatf test", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
			rep.Host = "https://adb-1111.1.azuredatabricks.net"
			if rep.Status != "complete" {
				t.Fatalf("status %s", rep.Status)
			}

			dir := t.TempDir()
			files, err := emit.WriteExport(dir, ex, rep, "workspace")
			if err != nil {
				t.Fatal(err)
			}
			goldenDir := filepath.Join("testdata", "golden", string(scope))
			if *update {
				_ = os.RemoveAll(goldenDir)
				if err := os.MkdirAll(goldenDir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			seen := map[string]bool{}
			for _, f := range files {
				name := filepath.Base(f)
				if name == "export-report.json" {
					continue
				}
				seen[name] = true
				got, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				goldenPath := filepath.Join(goldenDir, name)
				if *update {
					if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
						t.Fatal(err)
					}
					continue
				}
				want, err := os.ReadFile(goldenPath)
				if err != nil {
					t.Fatalf("missing golden %s (run with -update): %v", goldenPath, err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("%s differs from golden\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
				}
			}
			entries, _ := os.ReadDir(goldenDir)
			for _, e := range entries {
				if !seen[e.Name()] {
					t.Errorf("golden %s no longer produced", e.Name())
				}
			}
		})
	}
}

func TestSharedScopeHasNoWorkspaceVariables(t *testing.T) {
	srv := fakews.New(t)
	inv, err := inventory.New(srv.Client(t)).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ex, _ := contract.Build(inv, contract.ScopeShared)
	for _, v := range ex.Tfvars.Variables() {
		switch v.Name {
		case "cluster_policies", "instance_pools", "warehouses", "secret_scopes",
			"service_principals":
			t.Errorf("shared scope emitted %s", v.Name)
		}
	}
	if _, ok := ex.Tfvars.Catalogs["shared_ref"]; !ok {
		t.Error("shared scope should include OPEN catalog shared_ref")
	}
	if _, ok := ex.Tfvars.Catalogs["sales"]; ok {
		t.Error("shared scope must not include workspace-owned catalog sales")
	}
	if len(ex.Excluded) == 0 {
		t.Error("expected exclusions to be reported")
	}
}

func TestImportsMatchTfvars(t *testing.T) {
	srv := fakews.New(t)
	inv, err := inventory.New(srv.Client(t)).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ex, _ := contract.Build(inv, contract.ScopeWorkspace)
	keys := map[string]map[string]bool{}
	for _, imp := range ex.Imports {
		if keys[imp.Module] == nil {
			keys[imp.Module] = map[string]bool{}
		}
		keys[imp.Module][imp.Key] = true
	}
	for name := range ex.Tfvars.Catalogs {
		if !keys["catalog"][name] {
			t.Errorf("catalog %s has tfvars but no import", name)
		}
	}
	for name := range ex.Tfvars.ClusterPolicies {
		if !keys["cluster_policy"][name] {
			t.Errorf("policy %s has tfvars but no import", name)
		}
	}
	for key := range ex.Tfvars.WorkspaceBindings {
		if !keys["workspace_binding"][key] {
			t.Errorf("workspace binding %s has tfvars but no import", key)
		}
	}
	for _, imp := range ex.Imports {
		if imp.Module == "secret_scope" && imp.Resource == "databricks_secret_acl" {
			if _, ok := imp.Index.(string); !ok {
				t.Errorf("secret acl import must be keyed by principal: %+v", imp)
			}
		}
	}
}

func TestRenderImportsGroupsRepeatedTargets(t *testing.T) {
	imports := []contract.Import{
		{Module: "catalog", Key: "one", Resource: "databricks_catalog", ID: "one"},
		{Module: "catalog", Key: "two", Resource: "databricks_catalog", ID: "two"},
		{
			Module:   "secret_scope",
			Key:      "scope",
			Resource: "databricks_secret_acl",
			Index:    "group-one",
			ID:       "scope|||group-one",
		},
		{
			Module:   "secret_scope",
			Key:      "scope",
			Resource: "databricks_secret_acl",
			Index:    "group-two",
			ID:       "scope|||group-two",
		},
	}

	got := string(contract.RenderImports(imports, "workspace", nil))
	if count := strings.Count(got, "import {"); count != 2 {
		t.Fatalf("got %d import blocks, want 2:\n%s", count, got)
	}
	for _, want := range []string{
		"for_each = {",
		"module.workspace.module.catalog[each.key].databricks_catalog.this",
		"module.workspace.module.secret_scope[each.value.key].databricks_secret_acl.this[each.value.index]",
		"id = each.value",
		"id = each.value.id",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
}
