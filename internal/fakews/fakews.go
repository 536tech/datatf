// Package fakews serves recorded Databricks API fixtures over httptest so the
// reader and contract can be tested end to end without a workspace.
package fakews

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/databricks/databricks-sdk-go"
)

// WorkspaceID is the fixture workspace's id; OtherWorkspaceID is a sibling
// workspace used for multi-bound securables.
const (
	WorkspaceID      int64 = 1111
	OtherWorkspaceID int64 = 2222
)

// Route is one fixture endpoint. Query entries must all match; the most
// specific matching route wins.
type Route struct {
	Method string            `json:"method"`
	Path   string            `json:"path"`
	Query  map[string]string `json:"query,omitempty"`
	Status int               `json:"status,omitempty"`
	File   string            `json:"file,omitempty"`
	Body   json.RawMessage   `json:"body,omitempty"`
}

// Server is a fake workspace.
type Server struct {
	*httptest.Server
	mu     sync.Mutex
	routes []Route
	dir    string
}

// New starts a fake workspace from the manifest in testdata/routes.json.
func New(t testing.TB) *Server {
	t.Helper()
	s, err := load()
	if err != nil {
		t.Fatal(err)
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.Close)
	return s
}

// NewStandalone returns the fake as an http.Handler for fakewsd.
func NewStandalone() http.Handler {
	s, err := load()
	if err != nil {
		panic(err)
	}
	return http.HandlerFunc(s.handle)
}

func load() (*Server, error) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "testdata")
	raw, err := os.ReadFile(filepath.Join(dir, "routes.json"))
	if err != nil {
		return nil, fmt.Errorf("read routes manifest: %w", err)
	}
	s := &Server{dir: dir}
	if err := json.Unmarshal(raw, &s.routes); err != nil {
		return nil, fmt.Errorf("parse routes manifest: %w", err)
	}
	return s, nil
}

// Fail overrides a route to return an API error. It is matched before the
// manifest routes.
func (s *Server) Fail(method, path string, status int, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	body, _ := json.Marshal(map[string]string{"error_code": http.StatusText(status), "message": message})
	s.routes = append([]Route{{Method: method, Path: path, Status: status, Body: body}}, s.routes...)
}

// Client returns an SDK workspace client that talks to the fake.
func (s *Server) Client(t testing.TB) *databricks.WorkspaceClient {
	t.Helper()
	ws, err := databricks.NewWorkspaceClient(&databricks.Config{
		Host: s.URL, Token: "fake", AuthType: "pat", RetryTimeoutSeconds: 1,
		RateLimitPerSecond: 1000, // the SDK default of 15/s adds seconds per read
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return ws
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	routes := append([]Route{}, s.routes...)
	s.mu.Unlock()

	var best *Route
	for i := range routes {
		rt := &routes[i]
		if rt.Method != r.Method || rt.Path != r.URL.Path {
			continue
		}
		match := true
		for k, v := range rt.Query {
			if r.URL.Query().Get(k) != v {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		if best == nil || len(rt.Query) > len(best.Query) {
			best = rt
		}
	}
	if best == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error_code": "RESOURCE_DOES_NOT_EXIST",
			"message":    fmt.Sprintf("fakews: no fixture for %s %s", r.Method, r.URL.RequestURI()),
		})
		return
	}
	status := best.Status
	if status == 0 {
		status = http.StatusOK
	}
	body := []byte(best.Body)
	if best.File != "" {
		data, err := os.ReadFile(filepath.Join(s.dir, best.File))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		body = data
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
