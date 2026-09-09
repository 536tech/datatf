// Package inventory reads a Databricks workspace into a typed, read-only model
// and classifies every object by who should own it in Terraform.
package inventory

import (
	"github.com/databricks/databricks-sdk-go/service/catalog"
	"github.com/databricks/databricks-sdk-go/service/compute"
	"github.com/databricks/databricks-sdk-go/service/iam"
	"github.com/databricks/databricks-sdk-go/service/sql"
	"github.com/databricks/databricks-sdk-go/service/workspace"

	"github.com/536tech/datatf/internal/diagnostic"
)

// Ownership says which Terraform root should manage a Unity Catalog securable.
type Ownership string

const (
	// OwnedByWorkspace: ISOLATED and bound to exactly this workspace.
	OwnedByWorkspace Ownership = "workspace"
	// OwnedShared: OPEN, or ISOLATED and bound to zero or many workspaces.
	OwnedShared Ownership = "shared"
	// OwnedBySystem: Databricks-managed; never exported.
	OwnedBySystem Ownership = "system"
	// OwnedUnknown: ownership could not be verified; never exported.
	OwnedUnknown Ownership = "unknown"
)

// Issue is a read or build failure. Any issue makes the export partial.
type Issue struct {
	Area       string `json:"area"`
	ObjectName string `json:"object_name"`
	Operation  string `json:"operation"`
	Message    string `json:"message"`
	diagnostic.Details
}

// Skipped records an object that was seen and deliberately left out.
type Skipped struct {
	Area       string `json:"area"`
	ObjectName string `json:"object_name"`
	Reason     string `json:"reason"`
}

// Grant is one direct Unity Catalog privilege assignment.
type Grant struct {
	Principal  string   `json:"principal"`
	Privileges []string `json:"privileges"`
}

// Permission is one direct (non-inherited) workspace object permission.
type Permission struct {
	PermissionLevel      string `json:"permission_level"`
	GroupName            string `json:"group_name,omitempty"`
	UserName             string `json:"user_name,omitempty"`
	ServicePrincipalName string `json:"service_principal_name,omitempty"`
}

// Catalog is a managed catalog with its direct grants.
type Catalog struct {
	Info              catalog.CatalogInfo        `json:"info"`
	Ownership         Ownership                  `json:"ownership"`
	WorkspaceIDs      []int64                    `json:"workspace_ids,omitempty"`
	WorkspaceBindings []catalog.WorkspaceBinding `json:"workspace_bindings,omitempty"`
	GrantsRead        bool                       `json:"grants_read"`
	Grants            []Grant                    `json:"grants"`
	Schemas           []Schema                   `json:"schemas"`
}

// Schema is a user schema with its direct grants.
type Schema struct {
	Info       catalog.SchemaInfo `json:"info"`
	GrantsRead bool               `json:"grants_read"`
	Grants     []Grant            `json:"grants"`
}

// StorageCredential is a UC storage credential with its direct grants.
type StorageCredential struct {
	Info              catalog.StorageCredentialInfo `json:"info"`
	Ownership         Ownership                     `json:"ownership"`
	WorkspaceIDs      []int64                       `json:"workspace_ids,omitempty"`
	WorkspaceBindings []catalog.WorkspaceBinding    `json:"workspace_bindings,omitempty"`
	GrantsRead        bool                          `json:"grants_read"`
	Grants            []Grant                       `json:"grants"`
}

// ExternalLocation is a UC external location with its direct grants.
type ExternalLocation struct {
	Info              catalog.ExternalLocationInfo `json:"info"`
	Ownership         Ownership                    `json:"ownership"`
	WorkspaceIDs      []int64                      `json:"workspace_ids,omitempty"`
	WorkspaceBindings []catalog.WorkspaceBinding   `json:"workspace_bindings,omitempty"`
	GrantsRead        bool                         `json:"grants_read"`
	Grants            []Grant                      `json:"grants"`
}

// ClusterPolicy is a non-default cluster policy with its direct permissions.
type ClusterPolicy struct {
	Info        compute.Policy `json:"info"`
	Permissions []Permission   `json:"permissions"`
}

// InstancePool is an instance pool with its direct permissions.
type InstancePool struct {
	Info        compute.GetInstancePool `json:"info"`
	Permissions []Permission            `json:"permissions"`
}

// Warehouse is a SQL warehouse with its direct permissions.
type Warehouse struct {
	Info        sql.GetWarehouseResponse `json:"info"`
	Permissions []Permission             `json:"permissions"`
}

// SecretScope is a secret scope with its ACLs. Secret values are never read.
type SecretScope struct {
	Info     workspace.SecretScope `json:"info"`
	ACLsRead bool                  `json:"acls_read"`
	ACLs     []workspace.AclItem   `json:"acls"`
}

// ServicePrincipal is a workspace-level service principal.
type ServicePrincipal struct {
	Info         iam.ServicePrincipal `json:"info"`
	DisplayName  string               `json:"display_name"`
	Key          string               `json:"key"`
	Entitlements []string             `json:"entitlements"`
}

// Inventory contains the selected resource groups read from one workspace.
type Inventory struct {
	Host        string   `json:"host"`
	WorkspaceID int64    `json:"workspace_id"`
	MetastoreID string   `json:"metastore_id,omitempty"`
	UserName    string   `json:"user_name"`
	Resources   []string `json:"resources"`
	Name        *string  `json:"name,omitempty"`

	Catalogs           []Catalog           `json:"catalogs"`
	StorageCredentials []StorageCredential `json:"storage_credentials"`
	ExternalLocations  []ExternalLocation  `json:"external_locations"`
	ClusterPolicies    []ClusterPolicy     `json:"cluster_policies"`
	InstancePools      []InstancePool      `json:"instance_pools"`
	Warehouses         []Warehouse         `json:"warehouses"`
	SecretScopes       []SecretScope       `json:"secret_scopes"`
	ServicePrincipals  []ServicePrincipal  `json:"service_principals"`

	Issues  []Issue   `json:"issues"`
	Skipped []Skipped `json:"skipped"`
}

// Complete is true when no read failed. A partial inventory must not become
// Terraform without an explicit override.
func (inv *Inventory) Complete() bool {
	return len(inv.Issues) == 0
}
