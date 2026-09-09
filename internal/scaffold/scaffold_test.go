package scaffold

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/536tech/datatf/internal/contract"
)

func TestScaffoldSourcesAndProfile(t *testing.T) {
	for _, source := range []string{DefaultModuleSource, "../module", `C:\module`,
		"git::https://github.com/536tech/terraform-databricks-workspace.git?ref=v0.1.0",
		"registry.example.com/536tech/workspace/databricks"} {
		t.Run(source, func(t *testing.T) {
			opts := Options{Scope: contract.ScopeWorkspace, Host: "https://workspace.example.com",
				Profile: `profile"${1 + 1}`, RootModule: "workspace",
				ModuleSource: source, ModuleVersion: "~> 0.1"}
			files, err := Render(opts)
			if err != nil {
				t.Fatal(err)
			}
			module := parseBlock(t, files["main.tf"]).Body
			assertAttribute(t, module, "source", source)
			if (module.Attributes["version"] != nil) != registrySource.MatchString(source) {
				t.Fatal("version belongs only on registry sources")
			}
			provider := parseBlock(t, files["providers.tf"]).Body
			assertAttribute(t, provider, "profile", opts.Profile)
			assertAttribute(t, provider, "host", opts.Host)
		})
	}
}

func assertAttribute(t *testing.T, body *hclsyntax.Body, name, want string) {
	t.Helper()
	value, diags := body.Attributes[name].Expr.Value(nil)
	if diags.HasErrors() || value.AsString() != want {
		t.Fatalf("%s changed: %v %v", name, value, diags)
	}
}

func parseBlock(t *testing.T, source []byte) *hclsyntax.Block {
	t.Helper()
	file, diags := hclsyntax.ParseConfig(source, "generated.tf", hcl.InitialPos)
	if diags.HasErrors() {
		t.Fatal(diags)
	}
	return file.Body.(*hclsyntax.Body).Blocks[0]
}
