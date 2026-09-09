package cli

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/536tech/datatf/internal/fakews"
)

func isolateAuth(t *testing.T, srv *fakews.Server) {
	t.Helper()
	t.Setenv("DATABRICKS_CONFIG_FILE", filepath.Join(t.TempDir(), "missing.cfg"))
	t.Setenv("DATABRICKS_HOST", srv.URL)
	t.Setenv("DATABRICKS_TOKEN", "fake")
	t.Setenv("DATABRICKS_AUTH_TYPE", "pat")
	t.Setenv("DATABRICKS_CONFIG_PROFILE", "")
	t.Setenv("DATABRICKS_DISCOVERY_URL", srv.URL+"/.well-known/oauth-authorization-server")
	// The SDK default of 15 requests per second adds seconds per fake read.
	t.Setenv("DATABRICKS_RATE_LIMIT", "1000")
}

func TestExportEndToEnd(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	out := filepath.Join(t.TempDir(), "out")

	code, stdout, stderr := run(t, "export", "--quiet", "--out", out)
	if code != exitOK {
		t.Fatalf("exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	for _, name := range []string{"terraform.tfvars", "imports.tf", "export.json", "export-report.json"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("missing %s", name)
		}
	}
	autoTfvars, err := filepath.Glob(filepath.Join(out, "*.auto.tfvars"))
	if err != nil {
		t.Fatal(err)
	}
	if len(autoTfvars) != 0 {
		t.Errorf("unexpected auto tfvars: %v", autoTfvars)
	}
	tfvars, err := os.ReadFile(filepath.Join(out, "terraform.tfvars"))
	if err != nil {
		t.Fatal(err)
	}
	for _, variable := range []string{"catalogs =", "schemas =", "service_principals ="} {
		if !strings.Contains(string(tfvars), variable) {
			t.Errorf("terraform.tfvars missing %q", variable)
		}
	}
	if !strings.Contains(stdout, "status: complete") {
		t.Errorf("stdout = %s", stdout)
	}
}

func TestExportRefusesPartialWithoutFlag(t *testing.T) {
	srv := fakews.New(t)
	srv.Fail("GET", "/api/2.0/sql/warehouses", http.StatusInternalServerError, "boom")
	isolateAuth(t, srv)
	out := filepath.Join(t.TempDir(), "out")

	code, _, stderr := run(t, "export", "--quiet", "--out", out)
	if code != exitErr || !strings.Contains(stderr, "export is partial") {
		t.Fatalf("exit %d stderr %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(out, "imports.tf")); err == nil {
		t.Fatal("partial export must not write Terraform")
	}

	code, _, stderr = run(t, "export", "--quiet", "--out", out, "--allow-partial")
	if code != exitOK {
		t.Fatalf("allow-partial exit %d stderr %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(out, "imports.tf")); err != nil {
		t.Fatal("allow-partial should write Terraform")
	}
}

func TestInventoryEndToEnd(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	out := t.TempDir()
	code, stdout, stderr := run(t, "inventory", "--quiet", "--out", out)
	if code != exitOK {
		t.Fatalf("exit %d stderr %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(out, "inventory.json")); err != nil {
		t.Fatal("missing inventory.json")
	}
	if !strings.Contains(stdout, "catalogs") {
		t.Errorf("summary missing: %s", stdout)
	}
}

func TestAuthStatus(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	code, stdout, stderr := run(t, "auth", "status", "--plain")
	if code != exitOK || !strings.Contains(stdout, "jon@example.com") {
		t.Fatalf("exit %d stdout %s stderr %s", code, stdout, stderr)
	}
}

func TestDoctorIsUnavailable(t *testing.T) {
	code, _, stderr := run(t, "doctor")
	if code != exitUsage || !strings.Contains(stderr, `unknown command "doctor"`) {
		t.Fatalf("exit %d stderr %s", code, stderr)
	}
}

func TestExportScaffold(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	out := filepath.Join(t.TempDir(), "out")

	code, _, stderr := run(t, "export", "--quiet", "--out", out, "--scaffold", "--module-source", "../terraform-databricks-workspace")
	if code != exitOK {
		t.Fatalf("exit %d stderr %s", code, stderr)
	}
	main, err := os.ReadFile(filepath.Join(out, "main.tf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(main), `module "workspace"`) || strings.Contains(string(main), "version =") {
		t.Fatalf("main.tf = %s", main)
	}
	for _, name := range []string{"variables.tf", "providers.tf", "versions.tf", "README.md"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("missing %s", name)
		}
	}
	code, _, stderr = run(t, "export", "--quiet", "--out", out, "--scaffold")
	if code != exitErr || !strings.Contains(stderr, "refusing to overwrite") {
		t.Fatalf("second scaffold should refuse: %d %s", code, stderr)
	}
}

func TestExportHintsResourceLayout(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"selected group", []string{"--resources", "warehouses"}, true},
		{"selected name", []string{"--resources", "warehouses", "--name", "Analytics WH"}, true},
		{"default groups", nil, false},
		{"resource layout", []string{"--resources", "warehouses", "--module-layout", "resources"}, false},
		{"quiet", []string{"--resources", "warehouses", "--quiet"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"export", "--out", filepath.Join(t.TempDir(), "out")}, test.args...)
			code, _, stderr := run(t, args...)
			if code != exitOK {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			if got := strings.Contains(stderr, "--module-layout resources"); got != test.want {
				t.Fatalf("hint = %v, want %v: %s", got, test.want, stderr)
			}
		})
	}
}

func TestExportScaffoldRegistryDefault(t *testing.T) {
	srv := fakews.New(t)
	isolateAuth(t, srv)
	for _, scope := range []string{"workspace", "shared"} {
		t.Run(scope, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "out")
			code, _, stderr := run(t, "export", "--quiet", "--out", out,
				"--scope", scope, "--scaffold")
			if code != exitOK {
				t.Fatalf("exit %d stderr %s", code, stderr)
			}
			main, err := os.ReadFile(filepath.Join(out, "main.tf"))
			if err != nil {
				t.Fatal(err)
			}
			content := strings.Join(strings.Fields(string(main)), " ")
			for _, want := range []string{
				`source = "536tech/workspace/databricks"`, `version = "1.0.0"`,
			} {
				if !strings.Contains(content, want) {
					t.Errorf("main.tf missing %q:\n%s", want, main)
				}
			}
		})
	}
}
