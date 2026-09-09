package cli

import (
	"github.com/spf13/cobra"

	"github.com/536tech/datatf/internal/inventory"
)

func resourceSelection(cmd *cobra.Command, resources []string, shared bool) (
	[]string, *string, error,
) {
	selected, err := inventory.SelectResources(resources, shared)
	if err != nil {
		return nil, nil, err
	}
	if !cmd.Flags().Changed("name") {
		return selected, nil, nil
	}
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return nil, nil, err
	}
	return selected, &name, inventory.ValidateName(selected, &name)
}
