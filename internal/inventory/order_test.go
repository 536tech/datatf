package inventory_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/databricks/databricks-sdk-go"

	"github.com/536tech/datatf/internal/fakews"
	"github.com/536tech/datatf/internal/inventory"
)

// TestReadOrderIsDeterministic reads the same failing workspace twice, once
// with requests completing in arrival order and once with later arrivals
// completing first, and expects byte-identical inventories.
func TestReadOrderIsDeterministic(t *testing.T) {
	fixture := fakews.NewStandalone()
	readWith := func(delay func(arrival int64) time.Duration) []byte {
		var arrivals atomic.Int64
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(delay(arrivals.Add(1)))
			switch r.URL.Path {
			case "/api/2.1/unity-catalog/permissions/schema/sales.silver":
				http.Error(w, `{"error_code":"PERMISSION_DENIED","message":"no access"}`, http.StatusForbidden)
			case "/api/2.0/sql/warehouses":
				http.Error(w, `{"error_code":"INTERNAL_ERROR","message":"boom"}`, http.StatusInternalServerError)
			default:
				fixture.ServeHTTP(w, r)
			}
		}))
		defer srv.Close()
		ws, err := databricks.NewWorkspaceClient(&databricks.Config{
			Host: srv.URL, Token: "fake", AuthType: "pat", RetryTimeoutSeconds: 1, RateLimitPerSecond: 1000,
		})
		if err != nil {
			t.Fatal(err)
		}
		inv, err := inventory.New(ws).Read(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(inv.Issues) < 2 || len(inv.Skipped) == 0 {
			t.Fatalf("expected issues and skips, got %d issues, %d skipped", len(inv.Issues), len(inv.Skipped))
		}
		inv.Host = "" // each run has its own test server port
		data, err := json.Marshal(inv)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	inOrder := readWith(func(int64) time.Duration { return 0 })
	reversed := readWith(func(arrival int64) time.Duration {
		return time.Duration(max(0, 64-arrival)) * time.Millisecond
	})
	if string(inOrder) != string(reversed) {
		t.Fatalf("inventory depends on completion order\nin order: %s\nreversed: %s", inOrder, reversed)
	}
}
