package inventory_test

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/536tech/datatf/internal/contract"
	"github.com/536tech/datatf/internal/fakews"
	"github.com/536tech/datatf/internal/inventory"
)

func TestReadNamedResources(t *testing.T) {
	srv := fakews.New(t)
	full, issues := contract.Build(read(t, srv), contract.ScopeWorkspace)
	if len(issues) != 0 {
		t.Fatal(issues)
	}
	for _, tt := range []struct{ group, name string }{
		{"catalogs", "sales"},
		{"storage_credentials", "lake_cred"},
		{"external_locations", "lake_raw"},
		{"cluster_policies", "Team Policy"},
		{"instance_pools", "shared-pool"},
		{"warehouses", "Analytics WH"},
		{"secret_scopes", "db-scope"},
		{"service_principals", "etl-sp"},
		{"service_principals", "dup-sp (a1b2c3d4-0000-0000-0000-000000000002)"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := inventory.New(srv.Client(t))
			r.Resources, r.Name = []string{tt.group}, &tt.name
			inv, err := r.Read(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			ex, issues := contract.Build(inv, contract.ScopeWorkspace)
			report := contract.NewReport(inv, ex, issues, "test", time.Time{})
			assertNamedReport(t, report, tt.group, tt.name)
			for _, imp := range ex.Imports {
				if !slices.Contains(full.Imports, imp) {
					t.Fatalf("selection changed an import address or ID: %+v", imp)
				}
			}
		})
	}
}

func assertNamedReport(t *testing.T, report *contract.Report, group, name string) {
	t.Helper()
	if report.Status != "complete" || report.Counts[group] != 1 ||
		report.Name == nil || *report.Name != name || report.Imports == 0 {
		t.Fatalf("unexpected selection: %+v", report)
	}
}

func TestNamedReadDoesNotReadOtherObjects(t *testing.T) {
	for _, tt := range []struct{ group, name, blocked string }{
		{"catalogs", "sales", "/api/2.1/unity-catalog/catalogs/shared_ref"},
		{"storage_credentials", "lake_cred",
			"/api/2.1/unity-catalog/storage-credentials/shared_cred"},
		{"external_locations", "lake_raw", "/api/2.1/unity-catalog/external-locations/public_ref"},
	} {
		t.Run(tt.group, func(t *testing.T) {
			srv := fakews.New(t)
			srv.Fail("GET", tt.blocked, http.StatusForbidden, "denied")
			r := inventory.New(srv.Client(t))
			r.Resources, r.Name = []string{tt.group}, &tt.name
			inv, err := r.Read(context.Background())
			if err != nil || !inv.Complete() {
				t.Fatalf("selection read another object: %+v, %v", inv, err)
			}
		})
	}
}

func TestNamedReadKeepsOwnership(t *testing.T) {
	srv := fakews.New(t)
	r := inventory.New(srv.Client(t))
	name := "shared_ref"
	r.Resources, r.Name = []string{"catalogs"}, &name
	inv, err := r.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []contract.Scope{contract.ScopeWorkspace, contract.ScopeShared} {
		ex, issues := contract.Build(inv, scope)
		if len(issues) != 0 || !inv.Complete() {
			t.Fatalf("unexpected failure: %+v, %+v", issues, inv.Issues)
		}
		if (len(ex.Imports) > 0) != (scope == contract.ScopeShared) {
			t.Fatalf("selection bypassed %s scope: %+v", scope, ex.Imports)
		}
	}
}
