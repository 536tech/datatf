package contract

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"

	"github.com/536tech/datatf/internal/diagnostic"
	"github.com/536tech/datatf/internal/inventory"
	"github.com/databricks/databricks-sdk-go/service/catalog"
)

// Build selects the inventory objects that belong to scope and shapes them
// into tfvars plus matching import blocks. Imports are derived from the same
// selection, so the two never disagree.
func Build(inv *inventory.Inventory, scope Scope) (*Export, []inventory.Issue) {
	b := &builder{
		inv:   inv,
		scope: scope,
		ex: &Export{
			Scope:    scope,
			Tfvars:   newTfvars(scope),
			Imports:  []Import{},
			Excluded: []Exclusion{},
		},
	}
	b.catalogs()
	b.storageCredentials()
	b.externalLocations()
	if scope == ScopeWorkspace {
		b.clusterPolicies()
		b.instancePools()
		b.warehouses()
		b.secretScopes()
		b.servicePrincipals()
	}
	return b.ex, b.issues
}

func newTfvars(scope Scope) *Tfvars {
	t := &Tfvars{
		Catalogs:                map[string]CatalogSettings{},
		CatalogAccess:           map[string]Access{},
		Schemas:                 map[string][]string{},
		SchemaAccess:            map[string]map[string]Access{},
		SchemaStorageRoots:      map[string]map[string]string{},
		SchemaComments:          map[string]map[string]string{},
		StorageCredentials:      map[string]StorageCredentialSettings{},
		StorageCredentialAccess: map[string]Access{},
		ExternalLocations:       map[string]ExternalLocationSettings{},
		ExternalLocationAccess:  map[string]Access{},
		WorkspaceBindings:       map[string]WorkspaceBindingSettings{},
	}
	if scope == ScopeWorkspace {
		t.ClusterPolicies = map[string]ClusterPolicySettings{}
		t.InstancePools = map[string]InstancePoolSettings{}
		t.Warehouses = map[string]WarehouseSettings{}
		t.SecretScopes = map[string]SecretScopeSettings{}
		t.ServicePrincipals = map[string]PrincipalSettings{}
	}
	return t
}

type builder struct {
	inv    *inventory.Inventory
	scope  Scope
	ex     *Export
	issues []inventory.Issue
}

func (b *builder) issue(area, object, operation, message string) {
	b.issues = append(b.issues, inventory.Issue{
		Area: area, ObjectName: object, Operation: operation, Message: message,
		Details: diagnostic.Details{Code: "invalid_metadata",
			Hint: "Inspect the reported fields with datatf inventory --json. " +
				"Keep the same profile, host, and selection. Confirm the metadata with its owner."},
	})
}

// inScope reports whether a securable with the given ownership belongs to the
// export scope. Excluded objects are recorded for the report.
func (b *builder) inScope(area, name string, ownership inventory.Ownership) bool {
	switch {
	case ownership == inventory.OwnedByWorkspace && b.scope == ScopeWorkspace:
		return true
	case ownership == inventory.OwnedShared && b.scope == ScopeShared:
		return true
	case ownership == inventory.OwnedByWorkspace || ownership == inventory.OwnedShared:
		b.ex.Excluded = append(b.ex.Excluded, Exclusion{
			Area: area, Name: name, Ownership: string(ownership),
		})
	}
	return false
}

func (b *builder) addImport(module, key, resource string, index any, id string) {
	b.ex.Imports = append(b.ex.Imports, Import{
		Module: module, Key: key, Resource: resource, Index: index, ID: id,
	})
}

func toAccess(grants []inventory.Grant) Access {
	access := Access{}
	for _, g := range grants {
		if g.Principal == "" {
			continue
		}
		access[g.Principal] = append([]string{}, g.Privileges...)
	}
	return access
}

func toPermissions(perms []inventory.Permission) []Permission {
	out := make([]Permission, 0, len(perms))
	for _, p := range perms {
		out = append(out, Permission(p))
	}
	return out
}

func (b *builder) catalogs() {
	for _, cat := range b.inv.Catalogs {
		name := cat.Info.Name
		if !b.inScope("catalog", name, cat.Ownership) {
			continue
		}
		b.ex.Tfvars.Catalogs[name] = CatalogSettings{
			IsolationMode: string(cat.Info.IsolationMode),
			Owner:         cat.Info.Owner,
			Comment:       cat.Info.Comment,
			StorageRoot:   cat.Info.StorageRoot,
			Properties:    cat.Info.Properties,
		}
		b.addImport("catalog", name, "databricks_catalog", nil, name)
		b.workspaceBindings("catalog", name, cat.WorkspaceBindings)
		if cat.GrantsRead {
			access := toAccess(cat.Grants)
			b.ex.Tfvars.CatalogAccess[name] = access
			if len(access) > 0 {
				b.addImport("catalog", name, "databricks_grants", 0, "catalog/"+name)
			}
		}

		b.schemas(name, cat.Schemas)
	}
}

func (b *builder) schemas(name string, schemas []inventory.Schema) {
	schemaNames := []string{}
	for _, s := range schemas {
		schemaName := s.Info.Name
		if schemaName == "" {
			b.issue("unity_catalog", name, "build output", "Schema with empty name; skipped.")
			continue
		}
		fullName := name + "." + schemaName
		schemaNames = append(schemaNames, schemaName)
		b.addImport("schema", fullName, "databricks_schema", nil, fullName)
		if s.Info.StorageRoot != "" {
			ensureInner(b.ex.Tfvars.SchemaStorageRoots, name)[schemaName] = s.Info.StorageRoot
		}
		if s.Info.Comment != "" {
			ensureInner(b.ex.Tfvars.SchemaComments, name)[schemaName] = s.Info.Comment
		}
		if s.GrantsRead {
			access := toAccess(s.Grants)
			ensureInner(b.ex.Tfvars.SchemaAccess, name)[schemaName] = access
			if len(access) > 0 {
				b.addImport("schema", fullName, "databricks_grants", 0, "schema/"+fullName)
			}
		}
	}
	sort.Strings(schemaNames)
	b.ex.Tfvars.Schemas[name] = schemaNames
}

func (b *builder) workspaceBindings(
	securableType string,
	securableName string,
	bindings []catalog.WorkspaceBinding,
) {
	for _, binding := range bindings {
		bindingType := string(binding.BindingType)
		if bindingType == "" {
			bindingType = "BINDING_TYPE_READ_WRITE"
		}
		key := fmt.Sprintf("%d|%s|%s", binding.WorkspaceId, securableType, securableName)
		b.ex.Tfvars.WorkspaceBindings[key] = WorkspaceBindingSettings{
			WorkspaceID:   binding.WorkspaceId,
			SecurableName: securableName,
			SecurableType: securableType,
			BindingType:   bindingType,
		}
		b.addImport("workspace_binding", key, "databricks_workspace_binding", nil, key)
	}
}

func ensureInner[V any](outer map[string]map[string]V, key string) map[string]V {
	if outer[key] == nil {
		outer[key] = map[string]V{}
	}
	return outer[key]
}

func (b *builder) storageCredentials() {
	for _, sc := range b.inv.StorageCredentials {
		name := sc.Info.Name
		if !b.inScope("storage_credential", name, sc.Ownership) {
			continue
		}
		settings := StorageCredentialSettings{
			IsolationMode: string(sc.Info.IsolationMode),
			Owner:         sc.Info.Owner,
			Comment:       sc.Info.Comment,
			ReadOnly:      sc.Info.ReadOnly,
		}
		if mi := sc.Info.AzureManagedIdentity; mi != nil {
			settings.AzureManagedIdentity = &AzureManagedIdentity{
				AccessConnectorID: mi.AccessConnectorId,
				ManagedIdentityID: mi.ManagedIdentityId,
			}
		}
		b.ex.Tfvars.StorageCredentials[name] = settings
		b.addImport("storage_credential", name, "databricks_storage_credential", nil, name)
		b.workspaceBindings("storage_credential", name, sc.WorkspaceBindings)
		if sc.GrantsRead {
			access := toAccess(sc.Grants)
			b.ex.Tfvars.StorageCredentialAccess[name] = access
			if len(access) > 0 {
				b.addImport("storage_credential", name, "databricks_grants", 0, "storage_credential/"+name)
			}
		}
	}
}

func (b *builder) externalLocations() {
	for _, loc := range b.inv.ExternalLocations {
		name := loc.Info.Name
		if !b.inScope("external_location", name, loc.Ownership) {
			continue
		}
		b.ex.Tfvars.ExternalLocations[name] = ExternalLocationSettings{
			URL:              loc.Info.Url,
			CredentialName:   loc.Info.CredentialName,
			IsolationMode:    string(loc.Info.IsolationMode),
			Owner:            loc.Info.Owner,
			Comment:          loc.Info.Comment,
			ReadOnly:         loc.Info.ReadOnly,
			Fallback:         loc.Info.Fallback,
			EnableFileEvents: loc.Info.EnableFileEvents,
		}
		b.addImport("external_location", name, "databricks_external_location", nil, name)
		b.workspaceBindings("external_location", name, loc.WorkspaceBindings)
		if loc.GrantsRead {
			access := toAccess(loc.Grants)
			b.ex.Tfvars.ExternalLocationAccess[name] = access
			if len(access) > 0 {
				b.addImport("external_location", name, "databricks_grants", 0, "external_location/"+name)
			}
		}
	}
}

// parseJSONObject decodes an API JSON string into a generic value so it renders
// as HCL instead of an escaped string. Empty input returns nil.
func parseJSONObject(raw string) (any, error) {
	if raw == "" {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// roundTrip converts an SDK struct (or nil pointer) into a generic
// JSON-shaped value; nil pointers marshal to null and come back as nil.
func roundTrip(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}

func (b *builder) clusterPolicies() {
	for _, cp := range b.inv.ClusterPolicies {
		name := cp.Info.Name
		if name == "" || cp.Info.PolicyId == "" {
			b.issue("workspace", name, "build output", "Missing policy name or policy_id; skipped.")
			continue
		}
		settings := ClusterPolicySettings{
			Description:        cp.Info.Description,
			MaxClustersPerUser: cp.Info.MaxClustersPerUser,
			PolicyFamilyID:     cp.Info.PolicyFamilyId,
			Libraries:          []any{},
			Permissions:        toPermissions(cp.Permissions),
		}
		// Databricks requires exactly one of definition or policy_family_id.
		// Family-based policies expose a derived read-only definition; keep the
		// family id and overrides and drop the derived definition.
		raw, field := cp.Info.Definition, "policy definition"
		target := &settings.Definition
		if cp.Info.PolicyFamilyId != "" {
			raw, field = cp.Info.PolicyFamilyDefinitionOverrides, "policy family overrides"
			target = &settings.PolicyFamilyDefinitionOverrides
		}
		value, err := parseJSONObject(raw)
		if err != nil {
			b.issue("workspace", name, "build output",
				fmt.Sprintf("%s is not valid JSON: %v", field, err))
			continue
		}
		*target = value
		for _, lib := range cp.Info.Libraries {
			settings.Libraries = append(settings.Libraries, roundTrip(lib))
		}
		b.ex.Tfvars.ClusterPolicies[name] = settings
		b.addImport("cluster_policy", name, "databricks_cluster_policy", nil, cp.Info.PolicyId)
		if len(cp.Permissions) > 0 {
			b.addImport(
				"cluster_policy", name, "databricks_permissions", 0,
				"/cluster-policies/"+cp.Info.PolicyId,
			)
		}
	}
}

func (b *builder) instancePools() {
	for _, pool := range b.inv.InstancePools {
		name := pool.Info.InstancePoolName
		if name == "" || pool.Info.InstancePoolId == "" {
			b.issue("workspace", name, "build output", "Missing pool name or instance_pool_id; skipped.")
			continue
		}
		versions := []string{}
		for _, v := range pool.Info.PreloadedSparkVersions {
			if v != "" {
				versions = append(versions, v)
			}
		}
		b.ex.Tfvars.InstancePools[name] = InstancePoolSettings{
			NodeTypeID:                         pool.Info.NodeTypeId,
			MinIdleInstances:                   pool.Info.MinIdleInstances,
			MaxCapacity:                        pool.Info.MaxCapacity,
			IdleInstanceAutoterminationMinutes: pool.Info.IdleInstanceAutoterminationMinutes,
			EnableElasticDisk:                  pool.Info.EnableElasticDisk,
			PreloadedSparkVersions:             versions,
			CustomTags:                         pool.Info.CustomTags,
			AzureAttributes:                    roundTrip(pool.Info.AzureAttributes),
			Permissions:                        toPermissions(pool.Permissions),
		}
		b.addImport("instance_pool", name, "databricks_instance_pool", nil, pool.Info.InstancePoolId)
		if len(pool.Permissions) > 0 {
			b.addImport(
				"instance_pool", name, "databricks_permissions", 0,
				"/instance-pools/"+pool.Info.InstancePoolId,
			)
		}
	}
}

func (b *builder) warehouses() {
	for _, wh := range b.inv.Warehouses {
		name := wh.Info.Name
		if issue := warehouseIssue(wh); issue != "" {
			b.issue("workloads", name, "build output", issue)
			continue
		}
		tags := map[string]string{}
		if wh.Info.Tags != nil {
			for _, tag := range wh.Info.Tags.CustomTags {
				if tag.Key != "" {
					tags[tag.Key] = tag.Value
				}
			}
		}
		b.ex.Tfvars.Warehouses[name] = WarehouseSettings{
			ClusterSize:             wh.Info.ClusterSize,
			MinNumClusters:          wh.Info.MinNumClusters,
			MaxNumClusters:          wh.Info.MaxNumClusters,
			AutoStopMins:            wh.Info.AutoStopMins,
			WarehouseType:           string(wh.Info.WarehouseType),
			EnablePhoton:            wh.Info.EnablePhoton,
			EnableServerlessCompute: wh.Info.EnableServerlessCompute,
			SpotInstancePolicy:      string(wh.Info.SpotInstancePolicy),
			Tags:                    tags,
			Permissions:             toPermissions(wh.Permissions),
		}
		b.addImport("warehouse", name, "databricks_sql_endpoint", nil, wh.Info.Id)
		if len(wh.Permissions) > 0 {
			b.addImport("warehouse", name, "databricks_permissions", 0, "/sql/warehouses/"+wh.Info.Id)
		}
	}
}

func warehouseIssue(wh inventory.Warehouse) string {
	if wh.Info.Name == "" || wh.Info.Id == "" {
		return "Missing warehouse name or id; skipped."
	}
	if wh.Info.MaxNumClusters < 1 || wh.Info.WarehouseType == "" {
		return "Missing valid max_num_clusters or warehouse_type; read the full warehouse settings."
	}
	return ""
}

func (b *builder) secretScopes() {
	for _, scope := range b.inv.SecretScopes {
		name := scope.Info.Name
		settings := SecretScopeSettings{}
		if scope.ACLsRead {
			settings.ACLs = map[string]string{}
			for _, acl := range scope.ACLs {
				if acl.Principal == "" {
					b.issue("workspace", name, "build output", "Secret ACL with empty principal; skipped.")
					continue
				}
				settings.ACLs[acl.Principal] = string(acl.Permission)
			}
		}
		if kv := scope.Info.KeyvaultMetadata; kv != nil {
			settings.KeyvaultMetadata = &KeyvaultMetadata{ResourceID: kv.ResourceId, DNSName: kv.DnsName}
		}
		b.ex.Tfvars.SecretScopes[name] = settings
		b.addImport("secret_scope", name, "databricks_secret_scope", nil, name)
		principals := make([]string, 0, len(settings.ACLs))
		for p := range settings.ACLs {
			principals = append(principals, p)
		}
		sort.Strings(principals)
		for _, p := range principals {
			b.addImport("secret_scope", name, "databricks_secret_acl", p, name+"|||"+p)
		}
	}
}

func (b *builder) servicePrincipals() {
	for _, sp := range b.inv.ServicePrincipals {
		if sp.Key == "" || sp.Info.Id == "" {
			b.issue(
				"identity", sp.Key, "build output",
				"Missing service principal display name or SCIM id; skipped.",
			)
			continue
		}
		settings := PrincipalSettings{
			AllowClusterCreate:      slices.Contains(sp.Entitlements, "allow-cluster-create"),
			AllowInstancePoolCreate: slices.Contains(sp.Entitlements, "allow-instance-pool-create"),
			DatabricksSQLAccess:     slices.Contains(sp.Entitlements, "databricks-sql-access"),
			WorkspaceAccess:         slices.Contains(sp.Entitlements, "workspace-access"),
		}
		if sp.Key != sp.DisplayName {
			settings.DisplayName = sp.DisplayName
		}
		if slices.Contains(sp.Entitlements, "workspace-consume") {
			yes := true
			settings.WorkspaceConsume = &yes
		}
		b.ex.Tfvars.ServicePrincipals[sp.Key] = settings
		b.addImport("service_principal", sp.Key, "databricks_service_principal", nil, sp.Info.Id)
	}
}

func isNilMap(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Map && rv.IsNil()
}
