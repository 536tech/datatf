package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/databricks/databricks-sdk-go/service/catalog"
	"golang.org/x/sync/errgroup"
)

// classify decides which Terraform root owns a securable. A workspace root must
// not take ownership of shared metastore objects: only ISOLATED securables whose
// sole binding is this workspace are workspace-owned. Everything else is shared.
func (r *Reader) classify(
	ctx context.Context,
	securableType string,
	name string,
	isolationMode string,
) (Ownership, []int64, []catalog.WorkspaceBinding) {
	if r.inv.WorkspaceID == 0 {
		r.skip(
			"unity_catalog", name,
			"The workspace ID could not be resolved; ownership could not be verified.",
		)
		return OwnedUnknown, nil, nil
	}
	mode := strings.ToUpper(isolationMode)
	mode = strings.TrimPrefix(mode, "ISOLATION_MODE_")
	switch mode {
	case "", "OPEN":
		return OwnedShared, nil, nil
	case "ISOLATED":
	default:
		r.issue("unity_catalog", name, "workspace scope",
			fmt.Errorf("unknown isolation mode %q; ownership could not be verified", isolationMode))
		return OwnedUnknown, nil, nil
	}
	bindings, err := r.ws.WorkspaceBindings.GetBindingsAll(ctx, catalog.GetBindingsRequest{
		SecurableType: securableType,
		SecurableName: name,
	})
	if err != nil {
		r.issue("unity_catalog", name, "workspace-bindings get-bindings", err)
		return OwnedUnknown, nil, nil
	}
	bindings = sortedWorkspaceBindings(bindings)
	ids := make([]int64, 0, len(bindings))
	for _, b := range bindings {
		ids = append(ids, b.WorkspaceId)
	}
	ids = uniqueSortedInt64(ids)
	if len(ids) == 1 && ids[0] == r.inv.WorkspaceID {
		return OwnedByWorkspace, ids, bindings
	}
	return OwnedShared, ids, bindings
}

func sortedWorkspaceBindings(bindings []catalog.WorkspaceBinding) []catalog.WorkspaceBinding {
	out := append([]catalog.WorkspaceBinding{}, bindings...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].WorkspaceId == out[j].WorkspaceId {
			return string(out[i].BindingType) < string(out[j].BindingType)
		}
		return out[i].WorkspaceId < out[j].WorkspaceId
	})
	return out
}

// maxGrantPages bounds a hostile or looping paginated response.
const maxGrantPages = 1000

// grants reads direct privilege assignments on a securable, following pages.
func (r *Reader) grants(ctx context.Context, securableType, fullName string) ([]Grant, bool) {
	out := []Grant{}
	pageToken := ""
	for page := 1; ; page++ {
		resp, err := r.ws.Grants.Get(ctx, catalog.GetGrantRequest{
			SecurableType: securableType,
			FullName:      fullName,
			PageToken:     pageToken,
		})
		if err != nil {
			r.issue("unity_catalog", fullName, securableType+" grants", err)
			return nil, false
		}
		for _, pa := range resp.PrivilegeAssignments {
			privileges := make([]string, 0, len(pa.Privileges))
			for _, p := range pa.Privileges {
				privileges = append(privileges, string(p))
			}
			sort.Strings(privileges)
			out = append(out, Grant{Principal: pa.Principal, Privileges: privileges})
		}
		if resp.NextPageToken == "" {
			break
		}
		if resp.NextPageToken == pageToken || page >= maxGrantPages {
			r.issue("unity_catalog", fullName, securableType+" grants",
				fmt.Errorf("pagination did not finish after %d pages", page))
			return nil, false
		}
		pageToken = resp.NextPageToken
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Principal < out[j].Principal })
	return out, true
}

func (r *Reader) readCatalogs(ctx context.Context) {
	r.logf("Reading catalogs, schemas, and direct grants...")
	listed, err := r.ws.Catalogs.ListAll(ctx, catalog.ListCatalogsRequest{IncludeBrowse: true})
	if err != nil {
		r.issue("unity_catalog", "catalogs", "catalogs list", err)
		return
	}
	listed = selectNamed(r, listed, func(item catalog.CatalogInfo) string { return item.Name })
	results := make([]*Catalog, len(listed))
	g, gctx := r.group(ctx)
	for i, item := range listed {
		g.Go(func() error {
			results[i] = r.readCatalog(gctx, item)
			return nil
		})
	}
	_ = g.Wait()
	r.inv.Catalogs = compact(results)
}

func (r *Reader) readCatalog(ctx context.Context, item catalog.CatalogInfo) *Catalog {
	name := item.Name
	if item.CatalogType == "" {
		r.issue("unity_catalog", name, "catalogs list",
			fmt.Errorf("catalog_type is missing; the catalog type cannot be verified"))
		return nil
	}
	if item.CatalogType != catalog.CatalogTypeManagedCatalog {
		r.skip(
			"unity_catalog", name,
			fmt.Sprintf("Catalog type %s is not managed by this Terraform contract.", item.CatalogType),
		)
		return nil
	}
	if isSystemCatalog(name) {
		r.skip(
			"unity_catalog", name,
			"Databricks-managed/system catalog; owned by the platform, not this Terraform contract.",
		)
		return nil
	}
	// The get+classify chain, the grants, and the schema tree are independent.
	var info *catalog.CatalogInfo
	cat := &Catalog{Schemas: []Schema{}}
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		got, err := r.ws.Catalogs.Get(gctx, catalog.GetCatalogRequest{Name: name, IncludeBrowse: true})
		if err != nil {
			r.issue("unity_catalog", name, "catalogs get", err)
			return nil
		}
		info = got
		cat.Ownership, cat.WorkspaceIDs, cat.WorkspaceBindings = r.classify(
			gctx, "catalog", name, string(got.IsolationMode),
		)
		return nil
	})
	g.Go(func() error {
		cat.Grants, cat.GrantsRead = r.grants(gctx, "catalog", name)
		if cat.Grants == nil {
			cat.Grants = []Grant{}
		}
		return nil
	})
	g.Go(func() error { cat.Schemas = r.readSchemas(gctx, name); return nil })
	_ = g.Wait()
	if info == nil {
		return nil
	}
	cat.Info = *info
	r.logf("  Catalog: %s (%s)", name, cat.Ownership)
	return cat
}

func (r *Reader) readSchemas(ctx context.Context, catalogName string) []Schema {
	schemas, err := r.ws.Schemas.ListAll(ctx, catalog.ListSchemasRequest{
		CatalogName: catalogName, IncludeBrowse: true,
	})
	if err != nil {
		r.issue("unity_catalog", catalogName, "schemas list", err)
		return []Schema{}
	}
	results := make([]*Schema, len(schemas))
	g, gctx := r.group(ctx)
	for i, schema := range schemas {
		g.Go(func() error {
			results[i] = r.readSchema(gctx, schema)
			return nil
		})
	}
	_ = g.Wait()
	return compact(results)
}

func (r *Reader) readSchema(ctx context.Context, info catalog.SchemaInfo) *Schema {
	fullName := info.FullName
	if fullName == "" {
		fullName = info.CatalogName + "." + info.Name
	}
	if isSystemSchema(info.Name) {
		r.skip("unity_catalog", fullName, "System or auto-created schema.")
		return nil
	}
	s := &Schema{Info: info}
	s.Grants, s.GrantsRead = r.grants(ctx, "schema", fullName)
	if s.Grants == nil {
		s.Grants = []Grant{}
	}
	return s
}

func (r *Reader) readStorageCredentials(ctx context.Context) {
	r.logf("Reading storage credentials and direct grants...")
	listed, err := r.ws.StorageCredentials.ListAll(ctx, catalog.ListStorageCredentialsRequest{})
	if err != nil {
		r.issue("unity_catalog", "storage_credentials", "storage-credentials list", err)
		return
	}
	listed = selectNamed(r, listed, func(item catalog.StorageCredentialInfo) string { return item.Name })
	results := make([]*StorageCredential, len(listed))
	g, gctx := r.group(ctx)
	for i, item := range listed {
		g.Go(func() error {
			name := item.Name
			if isSystemStorageCredential(name) {
				r.skip(
					"unity_catalog", name,
					"Databricks-managed/system storage credential; "+
						"owned by the platform, not this Terraform contract.",
				)
				return nil
			}
			info, err := r.ws.StorageCredentials.GetByName(gctx, name)
			if err != nil {
				r.issue("unity_catalog", name, "storage-credentials get", err)
				return nil
			}
			ownership, ids, bindings := r.classify(
				gctx, "storage_credential", name, string(info.IsolationMode),
			)
			r.logf("  Storage credential: %s (%s)", name, ownership)
			sc := &StorageCredential{
				Info: redactStorageCredential(*info), Ownership: ownership,
				WorkspaceIDs: ids, WorkspaceBindings: bindings,
			}
			sc.Grants, sc.GrantsRead = r.grants(gctx, "storage_credential", name)
			if sc.Grants == nil {
				sc.Grants = []Grant{}
			}
			results[i] = sc
			return nil
		})
	}
	_ = g.Wait()
	r.inv.StorageCredentials = compact(results)
}

func (r *Reader) readExternalLocations(ctx context.Context) {
	r.logf("Reading external locations and direct grants...")
	listed, err := r.ws.ExternalLocations.ListAll(
		ctx, catalog.ListExternalLocationsRequest{IncludeBrowse: true},
	)
	if err != nil {
		r.issue("unity_catalog", "external_locations", "external-locations list", err)
		return
	}
	listed = selectNamed(r, listed, func(item catalog.ExternalLocationInfo) string { return item.Name })
	results := make([]*ExternalLocation, len(listed))
	g, gctx := r.group(ctx)
	for i, item := range listed {
		g.Go(func() error {
			name := item.Name
			if isSystemExternalLocation(name) {
				r.skip(
					"unity_catalog", name,
					"Databricks-managed/system external location; "+
						"owned by the platform, not this Terraform contract.",
				)
				return nil
			}
			info, err := r.ws.ExternalLocations.Get(gctx, catalog.GetExternalLocationRequest{
				Name: name, IncludeBrowse: true,
			})
			if err != nil {
				r.issue("unity_catalog", name, "external-locations get", err)
				return nil
			}
			ownership, ids, bindings := r.classify(
				gctx, "external_location", name, string(info.IsolationMode),
			)
			r.logf("  External location: %s (%s)", name, ownership)
			loc := &ExternalLocation{
				Info: *info, Ownership: ownership, WorkspaceIDs: ids, WorkspaceBindings: bindings,
			}
			loc.Grants, loc.GrantsRead = r.grants(gctx, "external_location", name)
			if loc.Grants == nil {
				loc.Grants = []Grant{}
			}
			results[i] = loc
			return nil
		})
	}
	_ = g.Wait()
	r.inv.ExternalLocations = compact(results)
}
