package inventory

import (
	"context"
	"testing"

	"github.com/databricks/databricks-sdk-go/service/catalog"
)

func TestMissingCatalogTypeIsPartial(t *testing.T) {
	r := New(nil)
	r.inv = &Inventory{}
	got := r.readCatalog(context.Background(), catalog.CatalogInfo{Name: "demo"})
	if got != nil || r.inv.Complete() || len(r.inv.Skipped) != 0 {
		t.Fatalf("missing catalog metadata must be an issue: %+v", r.inv)
	}
}
