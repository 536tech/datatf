package contract

import (
	"testing"

	"github.com/databricks/databricks-sdk-go/service/sql"

	"github.com/536tech/datatf/internal/inventory"
)

func TestWarehouseMissingSettingsBlocksExport(t *testing.T) {
	inv := &inventory.Inventory{Warehouses: []inventory.Warehouse{{
		Info: sql.GetWarehouseResponse{Id: "warehouse-1", Name: "demo", ClusterSize: "Small"},
	}}}
	ex, issues := Build(inv, ScopeWorkspace)
	if len(issues) != 1 || len(ex.Imports) != 0 || len(ex.Tfvars.Warehouses) != 0 {
		t.Fatalf("invalid warehouse became Terraform: %+v %+v", ex, issues)
	}
}
