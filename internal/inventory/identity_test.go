package inventory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/databricks/databricks-sdk-go"

	"github.com/536tech/datatf/internal/fakews"
)

func TestIdentityWithoutMetastore(t *testing.T) {
	fixture := fakews.NewStandalone()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/2.1/unity-catalog/current-metastore-assignment" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("X-Databricks-Org-Id", "1111")
		fixture.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	ws, err := databricks.NewWorkspaceClient(&databricks.Config{
		Host: srv.URL, Token: "fake", AuthType: "pat", RetryTimeoutSeconds: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	id, err := New(ws).Identify(context.Background())
	if err != nil || id.WorkspaceID != fakews.WorkspaceID || id.MetastoreID != "" {
		t.Fatalf("identity: %+v, error: %v", id, err)
	}
}
