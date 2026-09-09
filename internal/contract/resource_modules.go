package contract

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

// ResourceModule contains the selected inputs for one Registry resource module.
type ResourceModule struct {
	Name   string
	Source string
	Values cty.Value
}

type resourceModuleSpec struct {
	name   string
	values any
	access map[string]Access
	named  bool
}

// ResourceModules adapts the same selection used by Build to individual module inputs.
func ResourceModules(t *Tfvars) ([]ResourceModule, error) {
	specs := []resourceModuleSpec{
		{"catalog", t.Catalogs, t.CatalogAccess, true},
		{"schema", resourceSchemas(t), nil, false},
		{"storage_credential", t.StorageCredentials, t.StorageCredentialAccess, true},
		{"external_location", t.ExternalLocations, t.ExternalLocationAccess, true},
		{"workspace_binding", t.WorkspaceBindings, nil, false},
		{"cluster_policy", t.ClusterPolicies, nil, true},
		{"instance_pool", t.InstancePools, nil, true},
		{"warehouse", t.Warehouses, nil, true},
		{"secret_scope", t.SecretScopes, nil, true},
		{"service_principal", t.ServicePrincipals, nil, true},
	}
	modules := []ResourceModule{}
	for _, spec := range specs {
		values, err := resourceValues(spec)
		if err != nil {
			return nil, fmt.Errorf("build %s module inputs: %w", spec.name, err)
		}
		if values.LengthInt() == 0 {
			continue
		}
		registryName := strings.ReplaceAll(spec.name, "_", "-")
		if spec.name == "warehouse" {
			registryName = "sql-warehouse"
		}
		modules = append(modules, ResourceModule{
			Name: spec.name, Source: "registry.terraform.io/536tech/" + registryName + "/databricks",
			Values: values,
		})
	}
	return modules, nil
}

func resourceValues(spec resourceModuleSpec) (cty.Value, error) {
	values, err := toCty(spec.values)
	if err != nil {
		return cty.NilVal, err
	}
	if values.IsNull() {
		return cty.EmptyObjectVal, nil
	}
	objects := values.AsValueMap()
	for name, value := range objects {
		attributes := value.AsValueMap()
		if spec.named {
			attributes["name"] = cty.StringVal(name)
		}
		if spec.access != nil {
			attributes["grants"] = resourceGrants(spec.access[name])
		}
		objects[name] = cty.ObjectVal(attributes)
	}
	return cty.ObjectVal(objects), nil
}

func resourceGrants(access Access) cty.Value {
	grants := []cty.Value{}
	for _, principal := range slices.Sorted(maps.Keys(access)) {
		privileges := []cty.Value{}
		for _, privilege := range access[principal] {
			privileges = append(privileges, cty.StringVal(privilege))
		}
		grants = append(grants, cty.ObjectVal(map[string]cty.Value{
			"principal": cty.StringVal(principal), "privileges": cty.TupleVal(privileges),
		}))
	}
	return cty.TupleVal(grants)
}

func resourceSchemas(t *Tfvars) map[string]any {
	schemas := map[string]any{}
	for catalog, names := range t.Schemas {
		for _, name := range names {
			schemas[catalog+"."+name] = map[string]any{
				"catalog_name": catalog, "name": name,
				"storage_root": optionalString(t.SchemaStorageRoots[catalog][name]),
				"comment":      optionalString(t.SchemaComments[catalog][name]),
				"grants": ctyjson.SimpleJSONValue{
					Value: resourceGrants(t.SchemaAccess[catalog][name]),
				},
			}
		}
	}
	return schemas
}

func optionalString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
