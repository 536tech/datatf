package inventory

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/databricks/databricks-sdk-go"
	"github.com/databricks/databricks-sdk-go/service/catalog"
	"github.com/databricks/databricks-sdk-go/service/iam"
	"golang.org/x/sync/errgroup"

	"github.com/536tech/datatf/internal/diagnostic"
)

// Reader reads one workspace. It never writes to Databricks and never reads
// secret values.
type Reader struct {
	ws          *databricks.WorkspaceClient
	Parallelism int
	// Resources limits reads to the selected resource groups. Nil selects all groups.
	Resources []string
	// Name selects one exact name (service principal key) in one resource group.
	Name *string
	// Log receives progress lines. Nil disables progress.
	Log func(format string, args ...any)

	mu  sync.Mutex
	inv *Inventory
}

// New returns a Reader that fans each resource group out to 8 concurrent reads.
// Groups run concurrently; the SDK rate limit paces the total request rate.
func New(ws *databricks.WorkspaceClient) *Reader {
	return &Reader{ws: ws, Parallelism: 8}
}

// Read inventories the workspace. Authentication failure is fatal; every other
// failure is recorded as an Issue and reading continues.
func (r *Reader) Read(ctx context.Context) (*Inventory, error) {
	resources, err := SelectResources(r.Resources, false)
	if err != nil {
		return nil, err
	}
	if err := ValidateName(resources, r.Name); err != nil {
		return nil, err
	}
	r.inv = &Inventory{
		Host:               r.ws.Config.Host,
		Resources:          resources,
		Name:               r.Name,
		Catalogs:           []Catalog{},
		StorageCredentials: []StorageCredential{},
		ExternalLocations:  []ExternalLocation{},
		ClusterPolicies:    []ClusterPolicy{},
		InstancePools:      []InstancePool{},
		Warehouses:         []Warehouse{},
		SecretScopes:       []SecretScope{},
		ServicePrincipals:  []ServicePrincipal{},
		Issues:             []Issue{},
		Skipped:            []Skipped{},
	}

	r.logf("Reading workspace identity...")
	id, err := r.Identify(ctx)
	if err != nil {
		return nil, err
	}
	r.inv.UserName, r.inv.WorkspaceID, r.inv.MetastoreID = id.UserName, id.WorkspaceID, id.MetastoreID
	readers := map[string]func(context.Context){
		"catalogs": r.readCatalogs, "storage_credentials": r.readStorageCredentials,
		"external_locations": r.readExternalLocations, "cluster_policies": r.readClusterPolicies,
		"instance_pools": r.readInstancePools, "secret_scopes": r.readSecretScopes,
		"warehouses": r.readWarehouses, "service_principals": r.readServicePrincipals,
	}
	g, gctx := errgroup.WithContext(ctx)
	for _, name := range resources {
		g.Go(func() error { readers[name](gctx); return nil })
	}
	_ = g.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Groups interleave, so sort on every field for a deterministic report.
	slices.SortStableFunc(r.inv.Issues, func(a, b Issue) int {
		return cmp.Or(cmp.Compare(a.Area, b.Area), cmp.Compare(a.ObjectName, b.ObjectName),
			cmp.Compare(a.Operation, b.Operation), cmp.Compare(a.Message, b.Message),
			cmp.Compare(a.Code, b.Code), cmp.Compare(a.Hint, b.Hint),
			cmp.Compare(a.APICode, b.APICode), cmp.Compare(a.HTTPStatus, b.HTTPStatus))
	})
	slices.SortStableFunc(r.inv.Skipped, func(a, b Skipped) int {
		return cmp.Or(cmp.Compare(a.Area, b.Area), cmp.Compare(a.ObjectName, b.ObjectName),
			cmp.Compare(a.Reason, b.Reason))
	})
	return r.inv, nil
}

// Identity is who the client authenticates as and where.
type Identity struct {
	Host        string `json:"host"`
	UserName    string `json:"user_name"`
	AuthType    string `json:"auth_type"`
	WorkspaceID int64  `json:"workspace_id,omitempty"`
	MetastoreID string `json:"metastore_id,omitempty"`
}

// Identify authenticates and resolves the workspace's metastore assignment.
// The workspace ID does not require a Unity Catalog metastore assignment.
func (r *Reader) Identify(ctx context.Context) (*Identity, error) {
	var me *iam.User
	var assignment *catalog.MetastoreAssignment
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { me, err = r.ws.CurrentUser.Me(gctx, iam.MeRequest{}); return err })
	g.Go(func() error { assignment, _ = r.ws.Metastores.Current(gctx); return nil })
	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("authenticate against %s: %w", r.ws.Config.Host, err)
	}
	id := &Identity{Host: r.ws.Config.Host, UserName: me.UserName, AuthType: r.ws.Config.AuthType}
	if assignment != nil {
		id.WorkspaceID, id.MetastoreID = assignment.WorkspaceId, assignment.MetastoreId
	}
	var err error
	if id.WorkspaceID == 0 {
		id.WorkspaceID, err = r.ws.CurrentWorkspaceID(ctx)
		if err != nil {
			return nil, fmt.Errorf("resolve workspace id for %s: %w", id.Host, err)
		}
	}
	if id.WorkspaceID <= 0 {
		return nil, fmt.Errorf("no valid workspace id returned by %s; check the workspace URL", id.Host)
	}
	return id, nil
}

func (r *Reader) logf(format string, args ...any) {
	if r.Log != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.Log(format, args...)
	}
}

func (r *Reader) issue(area, object, operation string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	details := diagnostic.Describe(err)
	if area == "selection" {
		details = diagnostic.Details{Code: "selection_failed",
			Hint: "Use datatf inventory --json with the same profile, host, and resource group. " +
				"Omit --name to see the visible names and keys."}
	}
	r.inv.Issues = append(r.inv.Issues, Issue{
		Area: area, ObjectName: object, Operation: operation, Message: err.Error(),
		Details: details,
	})
}

func (r *Reader) skip(area, object, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inv.Skipped = append(r.inv.Skipped, Skipped{Area: area, ObjectName: object, Reason: reason})
}

// group returns an errgroup bounded by Parallelism. Workers report failures
// through issue(), so the group itself never returns an error.
func (r *Reader) group(ctx context.Context) (*errgroup.Group, context.Context) {
	g, gctx := errgroup.WithContext(ctx)
	limit := r.Parallelism
	if limit < 1 {
		limit = 1
	}
	g.SetLimit(limit)
	return g, gctx
}

// compact drops nil slots left by workers that skipped or failed, preserving
// list order so output is deterministic.
func compact[T any](items []*T) []T {
	out := make([]T, 0, len(items))
	for _, item := range items {
		if item != nil {
			out = append(out, *item)
		}
	}
	return out
}
