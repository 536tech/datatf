package inventory

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"
)

// Databricks-managed objects are owned by the platform, not by the target
// Terraform configuration. Databricks updates can recreate or change these objects.
// Some objects cannot be expressed in configuration. Excluding them keeps state stable.

func isSystemCatalog(name string) bool {
	return name == "main" || strings.HasPrefix(name, "__databricks")
}

func isSystemExternalLocation(name string) bool {
	return name == "metastore_root_location" || name == "metastore_default_location" ||
		strings.HasPrefix(name, "__databricks")
}

func isSystemStorageCredential(name string) bool {
	return strings.HasPrefix(name, "__databricks")
}

// Personal access-token scopes are user-scoped and ephemeral.
func isSystemSecretScope(name string) bool {
	return strings.HasSuffix(name, "-pat") || strings.HasPrefix(name, "__databricks")
}

func isSystemSchema(name string) bool {
	return name == "default" || name == "information_schema"
}

// Databricks-family default policies (Personal Compute, etc.) carry a
// policy_family_id and is_default.
func isDefaultFamilyPolicy(policyFamilyID string, isDefault bool) bool {
	return strings.TrimSpace(policyFamilyID) != "" && isDefault
}

// directPermissions flattens any permissions response into the entries the
// databricks_permissions resource will manage. It mirrors the provider's read:
// inherited entries, the "admins" group, and the calling identity (me) are
// dropped (the provider adds the caller as CAN_MANAGE itself and never tracks
// it), and principals are lowercased. The three SDK response types share a
// JSON shape, so one decoder serves all of them.
func directPermissions(accessControlList any, me string) []Permission {
	raw, err := json.Marshal(accessControlList)
	if err != nil {
		return nil
	}
	var entries []struct {
		Permission
		AllPermissions []struct {
			Inherited       bool   `json:"inherited"`
			PermissionLevel string `json:"permission_level"`
		} `json:"all_permissions"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil
	}
	out := []Permission{}
	for _, entry := range entries {
		if entry.implicit(me) {
			continue
		}
		for _, perm := range entry.AllPermissions {
			if perm.Inherited {
				continue
			}
			out = append(out, Permission{
				PermissionLevel:      perm.PermissionLevel,
				GroupName:            entry.GroupName,
				UserName:             strings.ToLower(entry.UserName),
				ServicePrincipalName: strings.ToLower(entry.ServicePrincipalName),
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].principal() != out[j].principal() {
			return out[i].principal() < out[j].principal()
		}
		return out[i].PermissionLevel < out[j].PermissionLevel
	})
	return out
}

func (p Permission) implicit(me string) bool {
	return p.GroupName == "admins" ||
		strings.EqualFold(p.UserName, me) || strings.EqualFold(p.ServicePrincipalName, me)
}

func (p Permission) principal() string {
	switch {
	case p.GroupName != "":
		return p.GroupName
	case p.UserName != "":
		return p.UserName
	default:
		return p.ServicePrincipalName
	}
}

func uniqueSortedInt64(values []int64) []int64 {
	out := append([]int64{}, values...)
	slices.Sort(out)
	return slices.Compact(out)
}
