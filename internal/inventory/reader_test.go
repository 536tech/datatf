package inventory_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/536tech/datatf/internal/fakews"
	"github.com/536tech/datatf/internal/inventory"
)

func read(t *testing.T, srv *fakews.Server) *inventory.Inventory {
	t.Helper()
	r := inventory.New(srv.Client(t))
	inv, err := r.Read(context.Background())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return inv
}

func TestReadCompleteWorkspace(t *testing.T) {
	srv := fakews.New(t)
	inv := read(t, srv)

	if !inv.Complete() {
		t.Fatalf("expected complete inventory, issues: %+v", inv.Issues)
	}
	if inv.WorkspaceID != fakews.WorkspaceID || inv.UserName != "jon@example.com" {
		t.Fatalf("identity not resolved: %+v", inv)
	}

	ownership := map[string]inventory.Ownership{}
	for _, c := range inv.Catalogs {
		ownership["catalog:"+c.Info.Name] = c.Ownership
	}
	for _, c := range inv.StorageCredentials {
		ownership["cred:"+c.Info.Name] = c.Ownership
	}
	for _, l := range inv.ExternalLocations {
		ownership["loc:"+l.Info.Name] = l.Ownership
	}
	want := map[string]inventory.Ownership{
		"catalog:sales":      inventory.OwnedByWorkspace,
		"catalog:shared_ref": inventory.OwnedShared,
		"cred:lake_cred":     inventory.OwnedByWorkspace,
		"cred:shared_cred":   inventory.OwnedShared,
		"loc:lake_raw":       inventory.OwnedByWorkspace,
		"loc:public_ref":     inventory.OwnedShared,
	}
	if len(ownership) != len(want) {
		t.Fatalf("ownership map = %v, want %v", ownership, want)
	}
	for k, v := range want {
		if ownership[k] != v {
			t.Errorf("%s ownership = %s, want %s", k, ownership[k], v)
		}
	}

	skipped := map[string]bool{}
	for _, s := range inv.Skipped {
		skipped[s.ObjectName] = true
	}
	for _, name := range []string{"main", "ext_share", "sales.default", "sales.information_schema",
		"__databricks_managed_cred", "metastore_root_location", "Personal Compute", "jon-pat"} {
		if !skipped[name] {
			t.Errorf("expected %q to be skipped; skipped=%v", name, skipped)
		}
	}

	var sales inventory.Catalog
	for _, c := range inv.Catalogs {
		if c.Info.Name == "sales" {
			sales = c
		}
	}
	if len(sales.Schemas) != 2 ||
		sales.Schemas[0].Info.Name != "bronze" ||
		sales.Schemas[1].Info.Name != "silver" {
		t.Fatalf("sales schemas = %+v", sales.Schemas)
	}
	if len(sales.Grants) != 2 || sales.Grants[0].Principal != "a1b2c3d4-0000-0000-0000-000000000001" {
		t.Fatalf("sales grants not sorted by principal: %+v", sales.Grants)
	}
	if got := sales.Grants[1].Privileges; len(got) != 3 || got[0] != "CREATE_SCHEMA" {
		t.Fatalf("privileges not sorted: %v", got)
	}

	var shared inventory.StorageCredential
	for _, c := range inv.StorageCredentials {
		if c.Info.Name == "shared_cred" {
			shared = c
		}
	}
	if len(shared.WorkspaceIDs) != 2 ||
		shared.WorkspaceIDs[0] != fakews.WorkspaceID ||
		shared.WorkspaceIDs[1] != fakews.OtherWorkspaceID {
		t.Fatalf("shared_cred bindings = %v", shared.WorkspaceIDs)
	}
	if len(shared.WorkspaceBindings) != 2 ||
		shared.WorkspaceBindings[0].WorkspaceId != fakews.WorkspaceID ||
		string(shared.WorkspaceBindings[0].BindingType) != "BINDING_TYPE_READ_WRITE" {
		t.Fatalf("shared_cred binding details = %+v", shared.WorkspaceBindings)
	}

	if len(inv.ClusterPolicies) != 2 {
		t.Fatalf("policies = %d, want 2 (default family policy skipped)", len(inv.ClusterPolicies))
	}
	for _, p := range inv.ClusterPolicies {
		for _, perm := range p.Permissions {
			if perm.GroupName == "admins" {
				t.Errorf("inherited permission leaked into policy %s: %+v", p.Info.Name, perm)
			}
		}
	}
	if len(inv.InstancePools) != 1 || len(inv.InstancePools[0].Permissions) != 0 {
		t.Fatalf("the calling user's own permission must be dropped: %+v", inv.InstancePools)
	}
	if len(inv.Warehouses) != 1 ||
		len(inv.Warehouses[0].Permissions) != 1 ||
		inv.Warehouses[0].Permissions[0].GroupName != "analysts" {
		t.Fatalf("direct admins entry must be dropped: %+v", inv.Warehouses)
	}
	if len(inv.SecretScopes) != 2 {
		t.Fatalf("secret scopes = %d, want 2", len(inv.SecretScopes))
	}

	keys := []string{}
	for _, sp := range inv.ServicePrincipals {
		keys = append(keys, sp.Key)
	}
	wantKeys := []string{
		"etl-sp",
		"dup-sp (a1b2c3d4-0000-0000-0000-000000000002)",
		"dup-sp (a1b2c3d4-0000-0000-0000-000000000003)",
	}
	if len(keys) != len(wantKeys) {
		t.Fatalf("service principal keys = %v", keys)
	}
	for i := range wantKeys {
		if keys[i] != wantKeys[i] {
			t.Errorf("sp key[%d] = %q, want %q", i, keys[i], wantKeys[i])
		}
	}
}

func TestReadRecordsIssuesAndContinues(t *testing.T) {
	srv := fakews.New(t)
	srv.Fail(
		"GET", "/api/2.1/unity-catalog/permissions/schema/sales.silver",
		http.StatusForbidden, "no access",
	)
	srv.Fail("GET", "/api/2.0/sql/warehouses", http.StatusInternalServerError, "boom")
	inv := read(t, srv)

	if inv.Complete() {
		t.Fatal("expected partial inventory")
	}
	ops := map[string]bool{}
	for _, issue := range inv.Issues {
		ops[issue.Operation+"|"+issue.ObjectName] = true
	}
	if !ops["schema grants|sales.silver"] || !ops["warehouses list|warehouses"] {
		t.Fatalf("issues = %+v", inv.Issues)
	}
	for _, c := range inv.Catalogs {
		if c.Info.Name != "sales" {
			continue
		}
		for _, s := range c.Schemas {
			if s.Info.Name == "silver" && s.GrantsRead {
				t.Fatal("silver grants should be marked unread")
			}
		}
	}
	if len(inv.ClusterPolicies) != 2 {
		t.Fatal("reading should continue past a failed area")
	}
}

func TestReadAuthFailureIsFatal(t *testing.T) {
	srv := fakews.New(t)
	srv.Fail("GET", "/api/2.0/preview/scim/v2/Me", http.StatusUnauthorized, "bad token")
	r := inventory.New(srv.Client(t))
	if _, err := r.Read(context.Background()); err == nil {
		t.Fatal("expected auth error")
	}
}
