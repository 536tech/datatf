package scaffold

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type registryTransport struct{ target *url.URL }

func (r registryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme, clone.URL.Host = r.target.Scheme, r.target.Host
	return http.DefaultTransport.RoundTrip(clone)
}

func TestLatestVersion(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		status           int
	}{
		{"semantic_order", `{"modules":[{"versions":[{"version":"1.9.0"},{"version":"1.10.0"},{"version":"2.0.0-rc.1"},{"version":"v9.0.0"},{"version":"8.0"},{"version":"invalid"}]}]}`, "1.10.0", 200},
		{"new_major_opt_in", `{"modules":[{"versions":[{"version":"2.0.0"},{"version":"1.0.0"}]}]}`, "2.0.0", 200},
		{"first_module_only", `{"modules":[{"versions":[{"version":"1.0.0"}]},{"versions":[{"version":"9.0.0"}]}]}`, "1.0.0", 200},
		{"build_metadata", `{"modules":[{"versions":[{"version":"1.9.0"},{"version":"1.10.0+build.1"}]}]}`, "1.10.0+build.1", 200},
		{"metadata_only", `{"modules":[{"versions":[{"version":"1.0.0+build.1"}]}]}`, "1.0.0+build.1", 200},
		{"metadata_tie", `{"modules":[{"versions":[{"version":"1.0.0+build.2"},{"version":"1.0.0+build.1"}]}]}`, "1.0.0+build.2", 200},
		{"empty", `{"modules":[]}`, "", 200},
		{"prerelease_only", `{"modules":[{"versions":[{"version":"1.0.0-beta"}]}]}`, "", 200},
		{"malformed", `{"modules":`, "", 200},
		{"missing", `private response text`, "", 404},
		{"rate_limited", `private response text`, "", 429},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/modules/536tech/workspace/databricks/versions" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				if r.Header.Get("Authorization") != "" {
					t.Error("Registry request must not contain workspace credentials")
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			target, _ := url.Parse(srv.URL)
			client := &http.Client{Transport: registryTransport{target}}
			got, err := latestVersion(context.Background(), client, "536tech/workspace/databricks")
			if tc.want == "" {
				if err == nil || strings.Contains(err.Error(), "private response text") {
					t.Fatalf("expected safe error: %v", err)
				}
			} else if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %s", got, err, tc.want)
			}
		})
	}
}

func TestLatestVersionCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := latestVersion(ctx, &http.Client{}, "536tech/workspace/databricks"); err == nil {
		t.Fatal("cancellation must stop resolution")
	}
}

func TestLatestSourceValidation(t *testing.T) {
	for _, source := range []string{"../local", "git::https://example.com/module.git", "private.example/536tech/workspace/databricks", "https://registry.terraform.io/536tech/workspace/databricks", "536tech/workspace/databricks//subdir"} {
		if PublicRegistryModule(source) {
			t.Fatalf("accepted %s", source)
		}
		if _, err := latestVersion(context.Background(), &http.Client{}, source); err == nil {
			t.Fatalf("resolved %s", source)
		}
	}
	for _, source := range []string{"536tech/workspace/databricks", "registry.terraform.io/536tech/sql-warehouse/databricks"} {
		if !PublicRegistryModule(source) {
			t.Fatalf("rejected %s", source)
		}
	}
}
