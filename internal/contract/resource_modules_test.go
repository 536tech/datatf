package contract

import (
	"strings"
	"testing"

	"github.com/zclconf/go-cty/cty"
)

func TestResourceModuleInputs(t *testing.T) {
	const name = `sales"${1+1}`
	tfvars := newTfvars(ScopeWorkspace)
	tfvars.Catalogs[name] = CatalogSettings{IsolationMode: "ISOLATED", Owner: "owner"}
	tfvars.CatalogAccess[name] = Access{"z": {"USE_CATALOG"}, "a": {"USE_CATALOG"}}
	tfvars.Schemas[name] = []string{"bronze"}
	tfvars.SchemaAccess[name] = map[string]Access{"bronze": {"analysts": {"USE_SCHEMA"}}}
	tfvars.WorkspaceBindings["binding"] = WorkspaceBindingSettings{WorkspaceID: 9007199254740993}
	modules, err := ResourceModules(tfvars)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]cty.Value{}
	for _, module := range modules {
		values[module.Name] = module.Values
	}
	catalog := values["catalog"].GetAttr(name)
	assertModuleValue(t, catalog.GetAttr("name"), cty.StringVal(name))
	assertModuleValue(t, catalog.GetAttr("grants").Index(cty.NumberIntVal(0)).GetAttr("principal"),
		cty.StringVal("a"))
	schema := values["schema"].GetAttr(name + ".bronze")
	assertModuleValue(t, schema.GetAttr("catalog_name"), cty.StringVal(name))
	assertModuleValue(t, schema.GetAttr("grants").Length(), cty.NumberIntVal(1))
	if !schema.GetAttr("storage_root").IsNull() || !schema.GetAttr("comment").IsNull() {
		t.Fatal("missing optional schema values must stay null")
	}
	if !values["workspace_binding"].GetAttr("binding").GetAttr("workspace_id").RawEquals(
		cty.NumberIntVal(9007199254740993)) {
		t.Fatal("workspace ID lost numeric precision")
	}
}

func assertModuleValue(t *testing.T, got, want cty.Value) {
	t.Helper()
	if !got.RawEquals(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestResourceModulesEmptyAndInvalid(t *testing.T) {
	modules, err := ResourceModules(newTfvars(ScopeShared))
	if err != nil || len(modules) != 0 {
		t.Fatalf("empty selection: %v %v", modules, err)
	}
	tfvars := newTfvars(ScopeWorkspace)
	tfvars.ClusterPolicies["invalid"] = ClusterPolicySettings{Definition: make(chan int)}
	_, err = ResourceModules(tfvars)
	if err == nil || !strings.Contains(err.Error(), "cluster_policy module inputs") {
		t.Fatalf("missing rendering context: %v", err)
	}
}
