// Package contract turns an inventory into the datatf Terraform module contract:
// tfvars values, import blocks, and the export report.
package contract

// Scope selects which Terraform root the export feeds.
type Scope string

const (
	// ScopeWorkspace: workspace-native resources plus UC securables that are
	// ISOLATED and bound only to this workspace.
	ScopeWorkspace Scope = "workspace"
	// ScopeShared: UC securables that are OPEN or bound to zero or many
	// workspaces. No workspace-native resources.
	ScopeShared Scope = "shared"
)

// Access is principal -> privileges.
type Access map[string][]string

// Permission is one direct permission on a workspace object. Exactly one
// principal field is set.
type Permission struct {
	PermissionLevel      string `json:"permission_level"`
	GroupName            string `json:"group_name,omitempty"`
	UserName             string `json:"user_name,omitempty"`
	ServicePrincipalName string `json:"service_principal_name,omitempty"`
}

type CatalogSettings struct {
	IsolationMode string            `json:"isolation_mode"`
	Owner         string            `json:"owner"`
	Comment       string            `json:"comment,omitempty"`
	StorageRoot   string            `json:"storage_root,omitempty"`
	Properties    map[string]string `json:"properties,omitempty"`
}

type AzureManagedIdentity struct {
	AccessConnectorID string `json:"access_connector_id"`
	ManagedIdentityID string `json:"managed_identity_id,omitempty"`
}

type StorageCredentialSettings struct {
	IsolationMode        string                `json:"isolation_mode"`
	Owner                string                `json:"owner"`
	Comment              string                `json:"comment,omitempty"`
	ReadOnly             bool                  `json:"read_only"`
	AzureManagedIdentity *AzureManagedIdentity `json:"azure_managed_identity,omitempty"`
}

type ExternalLocationSettings struct {
	URL              string `json:"url"`
	CredentialName   string `json:"credential_name"`
	IsolationMode    string `json:"isolation_mode"`
	Owner            string `json:"owner"`
	Comment          string `json:"comment,omitempty"`
	ReadOnly         bool   `json:"read_only"`
	Fallback         bool   `json:"fallback"`
	EnableFileEvents bool   `json:"enable_file_events"`
}

type WorkspaceBindingSettings struct {
	WorkspaceID   int64  `json:"workspace_id"`
	SecurableName string `json:"securable_name"`
	SecurableType string `json:"securable_type"`
	BindingType   string `json:"binding_type"`
}

type ClusterPolicySettings struct {
	Description                     string       `json:"description,omitempty"`
	Definition                      any          `json:"definition,omitempty"`
	PolicyFamilyID                  string       `json:"policy_family_id,omitempty"`
	PolicyFamilyDefinitionOverrides any          `json:"policy_family_definition_overrides,omitempty"`
	MaxClustersPerUser              int64        `json:"max_clusters_per_user,omitempty"`
	Libraries                       []any        `json:"libraries"`
	Permissions                     []Permission `json:"permissions"`
}

type InstancePoolSettings struct {
	NodeTypeID                         string            `json:"node_type_id"`
	MinIdleInstances                   int               `json:"min_idle_instances"`
	MaxCapacity                        int               `json:"max_capacity,omitempty"`
	IdleInstanceAutoterminationMinutes int               `json:"idle_instance_autotermination_minutes"`
	EnableElasticDisk                  bool              `json:"enable_elastic_disk"`
	PreloadedSparkVersions             []string          `json:"preloaded_spark_versions"`
	CustomTags                         map[string]string `json:"custom_tags,omitempty"`
	AzureAttributes                    any               `json:"azure_attributes,omitempty"`
	Permissions                        []Permission      `json:"permissions"`
}

type WarehouseSettings struct {
	ClusterSize             string            `json:"cluster_size"`
	MinNumClusters          int               `json:"min_num_clusters"`
	MaxNumClusters          int               `json:"max_num_clusters"`
	AutoStopMins            int               `json:"auto_stop_mins"`
	WarehouseType           string            `json:"warehouse_type"`
	EnablePhoton            bool              `json:"enable_photon"`
	EnableServerlessCompute bool              `json:"enable_serverless_compute"`
	SpotInstancePolicy      string            `json:"spot_instance_policy,omitempty"`
	Tags                    map[string]string `json:"tags,omitempty"`
	Permissions             []Permission      `json:"permissions"`
}

type KeyvaultMetadata struct {
	ResourceID string `json:"resource_id"`
	DNSName    string `json:"dns_name"`
}

type SecretScopeSettings struct {
	// ACLs is principal -> permission. Nil when the ACLs could not be read.
	ACLs             map[string]string `json:"acls,omitempty"`
	KeyvaultMetadata *KeyvaultMetadata `json:"keyvault_metadata,omitempty"`
}

type PrincipalSettings struct {
	// DisplayName is set only when the map key had to be disambiguated.
	DisplayName             string `json:"display_name,omitempty"`
	AllowClusterCreate      bool   `json:"allow_cluster_create"`
	AllowInstancePoolCreate bool   `json:"allow_instance_pool_create"`
	DatabricksSQLAccess     bool   `json:"databricks_sql_access"`
	WorkspaceAccess         bool   `json:"workspace_access"`
	// WorkspaceConsume is mutually exclusive with the two access flags in the
	// provider, so it is only present when granted.
	WorkspaceConsume *bool `json:"workspace_consume,omitempty"`
}

// Tfvars is every module input the exporter can populate. Workspace-native
// maps are nil for the shared scope and are not emitted.
type Tfvars struct {
	Catalogs                map[string]CatalogSettings           `json:"catalogs"`
	CatalogAccess           map[string]Access                    `json:"catalog_access"`
	Schemas                 map[string][]string                  `json:"schemas"`
	SchemaAccess            map[string]map[string]Access         `json:"schema_access"`
	SchemaStorageRoots      map[string]map[string]string         `json:"schema_storage_roots"`
	SchemaComments          map[string]map[string]string         `json:"schema_comments"`
	StorageCredentials      map[string]StorageCredentialSettings `json:"storage_credentials"`
	StorageCredentialAccess map[string]Access                    `json:"storage_credential_access"`
	ExternalLocations       map[string]ExternalLocationSettings  `json:"external_locations"`
	ExternalLocationAccess  map[string]Access                    `json:"external_location_access"`
	WorkspaceBindings       map[string]WorkspaceBindingSettings  `json:"workspace_bindings"`
	ClusterPolicies         map[string]ClusterPolicySettings     `json:"cluster_policies,omitempty"`
	InstancePools           map[string]InstancePoolSettings      `json:"instance_pools,omitempty"`
	Warehouses              map[string]WarehouseSettings         `json:"warehouses,omitempty"`
	SecretScopes            map[string]SecretScopeSettings       `json:"secret_scopes,omitempty"`
	ServicePrincipals       map[string]PrincipalSettings         `json:"service_principals,omitempty"`
}

// Variable is one tfvars block in emission order.
type Variable struct {
	Name    string
	Value   any
	Comment []string
}

// Variables returns the populated blocks in canonical order.
func (t *Tfvars) Variables() []Variable {
	all := []Variable{
		{"catalogs", t.Catalogs, []string{
			"Catalogs to manage. Key = catalog name; value = catalog settings.",
			"A catalog must be declared here before its schemas or grants can be added.",
		}},
		{"catalog_access", t.CatalogAccess, []string{
			"Direct grants ON a catalog. Shape: catalog name -> principal -> [privileges].",
			"Principals: groups/users by name or email; service principals by application id.",
			"Grants here are inherited by the catalog's schemas; do not repeat them in schema_access.",
		}},
		{"schemas", t.Schemas, []string{
			"Schemas to manage. Key = catalog name; value = list of schema names in that catalog.",
			"The catalog key must also exist in the catalogs block. " +
				"Empty list = catalog managed, no schemas.",
		}},
		{"schema_access", t.SchemaAccess, []string{
			"Direct grants ON a schema. Shape: catalog -> schema -> principal -> [privileges].",
			"Only needed for grants beyond what the schema already inherits from catalog_access.",
		}},
		{"schema_storage_roots", t.SchemaStorageRoots, []string{
			"Optional custom managed storage location per schema.",
			"Shape: catalog -> schema -> abfss URL. Only for schemas that do not use the catalog default.",
		}},
		{"schema_comments", t.SchemaComments, []string{
			"Optional schema descriptions. Shape: catalog -> schema -> comment string.",
		}},
		{"storage_credentials", t.StorageCredentials, []string{
			"Unity Catalog storage credentials. Key = credential name; value = settings",
			"(for Azure, an azure_managed_identity referencing an existing access connector).",
		}},
		{"storage_credential_access", t.StorageCredentialAccess, []string{
			"Direct grants ON a storage credential. Shape: credential name -> principal -> [privileges].",
		}},
		{"external_locations", t.ExternalLocations, []string{
			"External locations. Key = location name; value = settings including url and credential_name.",
			"credential_name can reference a credential managed here or an existing shared credential.",
		}},
		{"external_location_access", t.ExternalLocationAccess, []string{
			"Direct grants ON an external location. Shape: location name -> principal -> [privileges].",
		}},
		{"workspace_bindings", t.WorkspaceBindings, []string{
			"Unity Catalog workspace bindings. Key = provider import ID.",
			"Shape: <workspace_id>|<securable_type>|<securable_name> -> binding settings.",
		}},
		{"cluster_policies", t.ClusterPolicies, []string{
			"Cluster policies. Key = policy name; value = policy settings.",
			"Set exactly one of definition or policy_family_id. Permissions are nested inline.",
		}},
		{"instance_pools", t.InstancePools, []string{
			"Instance pools. Key = pool name; value = pool settings.",
			"Permissions are nested inline under each pool's permissions list.",
		}},
		{"warehouses", t.Warehouses, []string{
			"SQL warehouses. Key = warehouse name; value = warehouse settings.",
			"Permissions are nested inline under each warehouse's permissions list.",
		}},
		{"secret_scopes", t.SecretScopes, []string{
			"Secret scopes (prefer Key Vault-backed). Key = scope name; value = settings.",
			"ACLs are nested inline under acls: principal -> permission. Do not put secret values here.",
		}},
		{"service_principals", t.ServicePrincipals, []string{
			"Service principals. Key = display name; value = entitlements.",
			"Leave application_id unset for a new Databricks-managed one; Databricks generates it.",
		}},
	}
	out := make([]Variable, 0, len(all))
	for _, v := range all {
		if !isNilMap(v.Value) {
			out = append(out, v)
		}
	}
	return out
}

// Import is one Terraform import block addressed inside the composition module.
type Import struct {
	Module   string `json:"module"`
	Key      string `json:"key"`
	Resource string `json:"resource"`
	// Index is nil, an int (count-gated "[0]"), or a string (for_each key).
	Index any    `json:"index,omitempty"`
	ID    string `json:"id"`
}

// Exclusion is a UC securable that was read successfully but belongs to the
// other scope.
type Exclusion struct {
	Area      string `json:"area"`
	Name      string `json:"name"`
	Ownership string `json:"ownership"`
}

// Export is the complete result of one export.
type Export struct {
	Scope    Scope       `json:"scope"`
	Tfvars   *Tfvars     `json:"tfvars"`
	Imports  []Import    `json:"imports"`
	Excluded []Exclusion `json:"excluded"`
}
