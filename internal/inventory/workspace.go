package inventory

import (
	"context"
	"fmt"
	"strings"

	"github.com/databricks/databricks-sdk-go/service/compute"
	"github.com/databricks/databricks-sdk-go/service/iam"
	"github.com/databricks/databricks-sdk-go/service/sql"
	"github.com/databricks/databricks-sdk-go/service/workspace"
)

func (r *Reader) readClusterPolicies(ctx context.Context) {
	r.logf("Reading cluster policies and permissions...")
	listed, err := r.ws.ClusterPolicies.ListAll(ctx, compute.ListClusterPoliciesRequest{})
	if err != nil {
		r.issue("workspace", "cluster_policies", "cluster-policies list", err)
		return
	}
	listed = selectNamed(r, listed, func(item compute.Policy) string { return item.Name })
	results := make([]*ClusterPolicy, len(listed))
	g, gctx := r.group(ctx)
	for i, item := range listed {
		g.Go(func() error {
			info, err := r.ws.ClusterPolicies.GetByPolicyId(gctx, item.PolicyId)
			if err != nil {
				r.issue("workspace", item.Name, "cluster-policies get", err)
				return nil
			}
			if isDefaultFamilyPolicy(info.PolicyFamilyId, info.IsDefault) {
				r.skip(
					"workspace", item.Name,
					"Databricks-family default cluster policy; "+
						"owned by the platform, not this Terraform contract.",
				)
				return nil
			}
			r.logf("  Cluster policy: %s", item.Name)
			policy := &ClusterPolicy{Info: *info, Permissions: []Permission{}}
			perms, err := r.ws.ClusterPolicies.GetPermissions(
				gctx, compute.GetClusterPolicyPermissionsRequest{ClusterPolicyId: item.PolicyId},
			)
			if err != nil {
				r.issue("workspace", item.Name, "cluster policy permissions", err)
			} else {
				policy.Permissions = directPermissions(perms.AccessControlList, r.inv.UserName)
			}
			results[i] = policy
			return nil
		})
	}
	_ = g.Wait()
	r.inv.ClusterPolicies = compact(results)
}

func (r *Reader) readInstancePools(ctx context.Context) {
	r.logf("Reading instance pools and permissions...")
	listed, err := r.ws.InstancePools.ListAll(ctx)
	if err != nil {
		r.issue("workspace", "instance_pools", "instance-pools list", err)
		return
	}
	listed = selectNamed(r, listed, func(item compute.InstancePoolAndStats) string {
		return item.InstancePoolName
	})
	results := make([]*InstancePool, len(listed))
	g, gctx := r.group(ctx)
	for i, item := range listed {
		g.Go(func() error {
			info, err := r.ws.InstancePools.GetByInstancePoolId(gctx, item.InstancePoolId)
			if err != nil {
				r.issue("workspace", item.InstancePoolName, "instance-pools get", err)
				return nil
			}
			r.logf("  Instance pool: %s", item.InstancePoolName)
			pool := &InstancePool{Info: *info, Permissions: []Permission{}}
			perms, err := r.ws.InstancePools.GetPermissions(
				gctx, compute.GetInstancePoolPermissionsRequest{InstancePoolId: item.InstancePoolId},
			)
			if err != nil {
				r.issue("workspace", item.InstancePoolName, "instance pool permissions", err)
			} else {
				pool.Permissions = directPermissions(perms.AccessControlList, r.inv.UserName)
			}
			results[i] = pool
			return nil
		})
	}
	_ = g.Wait()
	r.inv.InstancePools = compact(results)
}

func (r *Reader) readSecretScopes(ctx context.Context) {
	r.logf("Reading secret scopes and ACLs...")
	listed, err := r.ws.Secrets.ListScopesAll(ctx)
	if err != nil {
		r.issue("workspace", "secret_scopes", "secrets list-scopes", err)
		return
	}
	listed = selectNamed(r, listed, func(item workspace.SecretScope) string { return item.Name })
	results := make([]*SecretScope, len(listed))
	g, gctx := r.group(ctx)
	for i, item := range listed {
		g.Go(func() error {
			name := item.Name
			if name == "" {
				return nil
			}
			if isSystemSecretScope(name) {
				r.skip(
					"workspace", name,
					"Personal access-token or system secret scope; not a durable platform object.",
				)
				return nil
			}
			r.logf("  Secret scope: %s", name)
			scope := &SecretScope{Info: item, ACLs: []workspace.AclItem{}}
			acls, err := r.ws.Secrets.ListAclsAll(gctx, workspace.ListAclsRequest{Scope: name})
			if err != nil {
				r.issue("workspace", name, "secrets list-acls", err)
			} else {
				scope.ACLsRead = true
				scope.ACLs = acls
			}
			results[i] = scope
			return nil
		})
	}
	_ = g.Wait()
	r.inv.SecretScopes = compact(results)
}

func (r *Reader) readWarehouses(ctx context.Context) {
	r.logf("Reading SQL warehouses and permissions...")
	listed, err := r.ws.Warehouses.ListAll(ctx, sql.ListWarehousesRequest{})
	if err != nil {
		r.issue("workloads", "warehouses", "warehouses list", err)
		return
	}
	listed = selectNamed(r, listed, func(item sql.EndpointInfo) string { return item.Name })
	results := make([]*Warehouse, len(listed))
	g, gctx := r.group(ctx)
	for i, item := range listed {
		g.Go(func() error {
			info, err := r.ws.Warehouses.GetById(gctx, item.Id)
			if err != nil {
				r.issue("workloads", item.Name, "warehouses get", err)
				return nil
			}
			r.logf("  Warehouse: %s", item.Name)
			wh := &Warehouse{Info: *info, Permissions: []Permission{}}
			perms, err := r.ws.Warehouses.GetPermissions(
				gctx, sql.GetWarehousePermissionsRequest{WarehouseId: item.Id},
			)
			if err != nil {
				r.issue("workloads", item.Name, "warehouse permissions", err)
			} else {
				wh.Permissions = directPermissions(perms.AccessControlList, r.inv.UserName)
			}
			results[i] = wh
			return nil
		})
	}
	_ = g.Wait()
	r.inv.Warehouses = compact(results)
}

// readServicePrincipals lists every workspace-level service principal. Display
// names are the tfvars map key but are not unique, so colliding names get an
// application-id suffix; unique names stay untouched.
func (r *Reader) readServicePrincipals(ctx context.Context) {
	r.logf("Reading service principals...")
	listed, err := r.ws.ServicePrincipalsV2.ListAll(ctx, iam.ListServicePrincipalsRequest{})
	if err != nil {
		r.issue("identity", "service_principals", "service-principals list", err)
		return
	}
	counts := map[string]int{}
	for _, sp := range listed {
		counts[spDisplayName(sp)]++
	}
	out := make([]ServicePrincipal, 0, len(listed))
	for _, sp := range listed {
		display := spDisplayName(sp)
		key := display
		if counts[display] > 1 {
			suffix := sp.ApplicationId
			if strings.TrimSpace(suffix) == "" {
				suffix = sp.Id
			}
			key = fmt.Sprintf("%s (%s)", display, suffix)
		}
		entitlements := make([]string, 0, len(sp.Entitlements))
		for _, e := range sp.Entitlements {
			entitlements = append(entitlements, e.Value)
		}
		out = append(out, ServicePrincipal{
			Info: sp, DisplayName: display, Key: key, Entitlements: entitlements,
		})
	}
	r.inv.ServicePrincipals = selectNamed(r, out, func(item ServicePrincipal) string { return item.Key })
	for _, sp := range r.inv.ServicePrincipals {
		r.logf("  Service principal: %s", sp.Key)
	}
}

func spDisplayName(sp iam.ServicePrincipal) string {
	if strings.TrimSpace(sp.DisplayName) != "" {
		return sp.DisplayName
	}
	return sp.ApplicationId
}
