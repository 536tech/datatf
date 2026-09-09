package contract

import (
	"time"

	"github.com/536tech/datatf/internal/inventory"
)

// Report is export-report.json. Status "partial" means at least one read or
// build step failed and the Terraform output is incomplete.
type Report struct {
	Status      string              `json:"status"`
	ExportedAt  time.Time           `json:"exported_at"`
	Tool        string              `json:"tool"`
	Host        string              `json:"host"`
	UserName    string              `json:"user_name"`
	WorkspaceID int64               `json:"workspace_id"`
	MetastoreID string              `json:"metastore_id,omitempty"`
	Scope       Scope               `json:"scope,omitempty"`
	Resources   []string            `json:"resources"`
	Name        *string             `json:"name,omitempty"`
	Counts      map[string]int      `json:"counts"`
	Imports     int                 `json:"imports"`
	Issues      []inventory.Issue   `json:"issues"`
	Skipped     []inventory.Skipped `json:"skipped"`
	Excluded    []Exclusion         `json:"excluded"`
	Limitations map[string]string   `json:"limitations"`
}

// NewReport summarizes an inventory and, when ex is non-nil, the export built
// from it. buildIssues are issues raised while shaping tfvars.
func NewReport(inv *inventory.Inventory, ex *Export, buildIssues []inventory.Issue, tool string, now time.Time) *Report {
	issues := append([]inventory.Issue{}, inv.Issues...)
	issues = append(issues, buildIssues...)
	status := "complete"
	if len(issues) > 0 {
		status = "partial"
	}
	rep := &Report{
		Status:      status,
		ExportedAt:  now.UTC(),
		Tool:        tool,
		Host:        inv.Host,
		UserName:    inv.UserName,
		WorkspaceID: inv.WorkspaceID,
		MetastoreID: inv.MetastoreID,
		Resources:   inv.Resources,
		Name:        inv.Name,
		Counts:      map[string]int{},
		Issues:      issues,
		Skipped:     append([]inventory.Skipped{}, inv.Skipped...),
		Excluded:    []Exclusion{},
		Limitations: map[string]string{
			"workspace_settings": "The workspace-conf API cannot enumerate unknown keys; workspace settings are not exported.",
			"terraform_controls": "force_destroy, force_update, and skip_validation are Terraform choices, not live state.",
			"account_resources":  "Account-level Databricks resources and cloud (Azure) resources are out of scope.",
			"secret_values":      "Secret values are never read or exported.",
			"policy_json_order":  "Cluster policy definitions are emitted as HCL objects; Terraform's jsonencode sorts keys, so the first apply may rewrite a definition with identical content in sorted key order (shown as an in-place update with '# whitespace changes'). Later plans are clean.",
		},
	}
	if ex == nil {
		rep.Counts["catalogs"] = len(inv.Catalogs)
		rep.Counts["storage_credentials"] = len(inv.StorageCredentials)
		rep.Counts["external_locations"] = len(inv.ExternalLocations)
		rep.Counts["cluster_policies"] = len(inv.ClusterPolicies)
		rep.Counts["instance_pools"] = len(inv.InstancePools)
		rep.Counts["warehouses"] = len(inv.Warehouses)
		rep.Counts["secret_scopes"] = len(inv.SecretScopes)
		rep.Counts["service_principals"] = len(inv.ServicePrincipals)
		return rep
	}
	rep.Scope = ex.Scope
	rep.Imports = len(ex.Imports)
	rep.Excluded = ex.Excluded
	t := ex.Tfvars
	rep.Counts["catalogs"] = len(t.Catalogs)
	schemas := 0
	for _, names := range t.Schemas {
		schemas += len(names)
	}
	rep.Counts["schemas"] = schemas
	rep.Counts["storage_credentials"] = len(t.StorageCredentials)
	rep.Counts["external_locations"] = len(t.ExternalLocations)
	rep.Counts["workspace_bindings"] = len(t.WorkspaceBindings)
	rep.Counts["cluster_policies"] = len(t.ClusterPolicies)
	rep.Counts["instance_pools"] = len(t.InstancePools)
	rep.Counts["warehouses"] = len(t.Warehouses)
	rep.Counts["secret_scopes"] = len(t.SecretScopes)
	rep.Counts["service_principals"] = len(t.ServicePrincipals)
	return rep
}
