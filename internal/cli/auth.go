package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/536tech/datatf/internal/inventory"
)

func newAuthCommand(rc *runtime) *cobra.Command {
	auth := &cobra.Command{
		Use:                        "auth",
		Args:                       usageArgs(cobra.NoArgs),
		Short:                      "Inspect Databricks authentication",
		SuggestFor:                 []string{"doctor"},
		SuggestionsMinimumDistance: 2,
		RunE: func(cmd *cobra.Command, args []string) error {
			return commandRequired(cmd, rc)
		},
	}
	auth.AddCommand(newAuthStatusCommand(rc))
	return auth
}

func newAuthStatusCommand(rc *runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Args:  usageArgs(cobra.NoArgs),
		Short: "Verify authentication and show the resolved workspace",
		Long: `Verify authentication and show the resolved workspace.
This command does not log in or prove access to every resource.
Use inventory --json to check the visible resource selection.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := rc.client()
			if err != nil {
				return err
			}
			id, err := inventory.New(ws).Identify(cmd.Context())
			if err != nil {
				return workspaceError(err)
			}
			if rc.out.IsJSON() {
				return rc.out.JSON(id)
			}
			rc.out.Success("authenticated")
			rc.out.Table([]string{"KEY", "VALUE"}, [][]string{
				{"host", id.Host},
				{"user", id.UserName},
				{"auth_type", id.AuthType},
				{"workspace_id", fmt.Sprint(id.WorkspaceID)},
				{"metastore_id", id.MetastoreID},
			})
			return nil
		},
	}
}
