package inventory

import (
	"fmt"
	"slices"
	"strings"
)

// ResourceNames lists the resource groups accepted by the reader.
var ResourceNames = []string{
	"catalogs", "storage_credentials", "external_locations", "cluster_policies",
	"instance_pools", "warehouses", "secret_scopes", "service_principals",
}

// SelectResources validates a selection and returns it in canonical order.
// Nil selects every available group. Shared scope includes only UC groups.
func SelectResources(names []string, shared bool) ([]string, error) {
	available := ResourceNames
	if shared {
		available = available[:3]
	}
	if len(names) == 0 {
		if names != nil {
			return nil, fmt.Errorf("--resources needs at least one resource group")
		}
		return slices.Clone(available), nil
	}
	for _, name := range names {
		if !slices.Contains(available, name) {
			return nil, fmt.Errorf("invalid resource %q; choose from %s", name,
				strings.Join(available, ", "))
		}
	}
	selected := []string{}
	for _, name := range available {
		if slices.Contains(names, name) {
			selected = append(selected, name)
		}
	}
	return selected, nil
}

// ValidateName requires one group and a non-empty name for object selection.
// Nil selects every visible object in the selected groups.
func ValidateName(resources []string, name *string) error {
	if name == nil {
		return nil
	}
	if strings.TrimSpace(*name) == "" {
		return fmt.Errorf("--name must not be empty; use an exact object name")
	}
	if len(resources) != 1 {
		return fmt.Errorf("--name needs exactly one resource group in --resources")
	}
	return nil
}

func selectNamed[T any](r *Reader, items []T, name func(T) string) []T {
	if r.Name == nil {
		return items
	}
	selected := []T{}
	for _, item := range items {
		if name(item) == *r.Name {
			selected = append(selected, item)
		}
	}
	if len(selected) != 1 {
		r.issue("selection", *r.Name, "select name", fmt.Errorf(
			"found %d visible matches in %s; use inventory --json to check the exact name or key",
			len(selected), r.inv.Resources[0]))
		return nil
	}
	return selected
}
