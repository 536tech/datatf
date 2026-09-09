package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/536tech/datatf/internal/contract"
	"github.com/536tech/datatf/internal/emit"
	"github.com/536tech/datatf/internal/inventory"
)

func newInventoryCommand(rc *runtime) *cobra.Command {
	var outDir string
	var resources []string
	cmd := &cobra.Command{
		Use:   "inventory",
		Args:  usageArgs(cobra.NoArgs),
		Short: "Read supported platform configuration",
		Long: `inventory reads supported platform configuration and writes
inventory.json plus inventory-report.json. Each supported Unity Catalog
securable is classified as workspace-owned, shared, or system so you can see
what an export would include before you run one.

Use --resources to select groups (see export --help).
Use --name with one group to select one exact, case-sensitive name.
For service principals, use the key from inventory --json.

--json writes the inventory to stdout without local files.
An incomplete inventory still returns its data, but exits with code 1.`,
		Example: `  datatf inventory --profile analytics --resources catalogs --json
  datatf inventory --profile analytics --resources catalogs --name sales --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			selected, name, err := resourceSelection(cmd, resources, false)
			if err != nil {
				return fmt.Errorf("%w: %v", errUsage, err)
			}
			rc.usage.ResourceGroups = selected
			ws, err := rc.client()
			if err != nil {
				return err
			}
			reader := inventory.New(ws)
			reader.Resources = selected
			reader.Name = name
			reader.Log = rc.progress()
			inv, err := reader.Read(cmd.Context())
			if err != nil {
				return workspaceError(err)
			}
			rep := contract.NewReport(inv, nil, nil, "datatf "+version, now())
			if rc.out.IsJSON() {
				if err := rc.out.JSON(inv); err != nil {
					return err
				}
				return rc.partialError("inventory", rep.Issues)
			}
			files, err := emit.WriteInventory(outDir, inv, rep)
			if err != nil {
				return outputError(err)
			}
			writeInventorySummary(rc, inv, rep, files)
			return rc.partialError("inventory", rep.Issues)
		},
	}
	cmd.Flags().StringVarP(&outDir, "out", "o", ".", "output directory")
	cmd.Flags().StringSliceVar(&resources, "resources", nil,
		"limit reads to resource groups (comma-separated; see export --help)")
	cmd.Flags().String("name", "", "select one exact name within one --resources group")
	return cmd
}

func writeInventorySummary(rc *runtime, inv *inventory.Inventory, rep *contract.Report, files []string) {
	rc.out.Println()
	rc.out.Table([]string{"AREA", "WORKSPACE", "SHARED", "TOTAL"}, summarize(inv))
	rc.out.Println()
	for _, f := range files {
		rc.out.Printf("wrote %s\n", f)
	}
	rc.out.Printf("status: %s (%d issues, %d skipped)\n", rep.Status, len(rep.Issues), len(rep.Skipped))
}

// summarize counts UC securables by ownership; workspace-native kinds are
// always workspace-owned.
func summarize(inv *inventory.Inventory) [][]string {
	var owned []inventory.Ownership
	ucRow := func(area string) []string {
		w, s := 0, 0
		for _, o := range owned {
			switch o {
			case inventory.OwnedByWorkspace:
				w++
			case inventory.OwnedShared:
				s++
			}
		}
		return []string{area, fmt.Sprint(w), fmt.Sprint(s), fmt.Sprint(len(owned))}
	}
	wsRow := func(area string, n int) []string {
		return []string{area, fmt.Sprint(n), "0", fmt.Sprint(n)}
	}
	owned = owned[:0]
	for _, c := range inv.Catalogs {
		owned = append(owned, c.Ownership)
	}
	rows := [][]string{ucRow("catalogs")}
	owned = owned[:0]
	for _, c := range inv.StorageCredentials {
		owned = append(owned, c.Ownership)
	}
	rows = append(rows, ucRow("storage_credentials"))
	owned = owned[:0]
	for _, l := range inv.ExternalLocations {
		owned = append(owned, l.Ownership)
	}
	rows = append(rows, ucRow("external_locations"))
	return append(rows,
		wsRow("cluster_policies", len(inv.ClusterPolicies)),
		wsRow("instance_pools", len(inv.InstancePools)),
		wsRow("warehouses", len(inv.Warehouses)),
		wsRow("secret_scopes", len(inv.SecretScopes)),
		wsRow("service_principals", len(inv.ServicePrincipals)),
	)
}
