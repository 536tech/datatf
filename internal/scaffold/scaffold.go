// Package scaffold renders a Terraform root for the DataTF module contract.
package scaffold

import (
	"fmt"
	"regexp"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"

	"github.com/536tech/datatf/internal/contract"
)

// DefaultModuleSource identifies the published workspace module contract.
const DefaultModuleSource = "536tech/workspace/databricks"

// DefaultModuleVersion pins the tested Registry release for both layouts.
const DefaultModuleVersion = "1.0.0"

// Options control the generated root.
type Options struct {
	Scope         contract.Scope
	Host          string
	Profile       string
	RootModule    string
	ModuleSource  string
	ModuleVersion string
}

var variableNames = []string{
	"catalogs", "catalog_access", "schemas", "schema_access", "schema_storage_roots",
	"schema_comments", "storage_credentials", "storage_credential_access",
	"external_locations", "external_location_access", "workspace_bindings", "cluster_policies",
	"instance_pools", "warehouses", "secret_scopes", "service_principals",
}

var workspaceOnly = map[string]bool{
	"cluster_policies": true, "instance_pools": true, "warehouses": true,
	"secret_scopes": true, "service_principals": true,
}

var registrySource = regexp.MustCompile(
	`^(?:[a-zA-Z0-9.-]+(?::[0-9]+)?/)?[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+$`,
)

// Render builds every scaffold file before the caller writes the export.
func Render(opts Options) (map[string][]byte, error) {
	if !hclsyntax.ValidIdentifier(opts.RootModule) {
		return nil, fmt.Errorf("scaffold needs a valid root module name; pass --root-module")
	}
	if opts.ModuleSource == "" {
		opts.ModuleSource = DefaultModuleSource
	}
	main := hclwrite.NewEmptyFile()
	module := main.Body().AppendNewBlock("module", []string{opts.RootModule}).Body()
	module.SetAttributeValue("source", cty.StringVal(opts.ModuleSource))
	if registrySource.MatchString(opts.ModuleSource) && opts.ModuleVersion != "" {
		module.SetAttributeValue("version", cty.StringVal(opts.ModuleVersion))
	}
	variables := hclwrite.NewEmptyFile()
	for _, name := range variableNames {
		if opts.Scope == contract.ScopeShared && workspaceOnly[name] {
			continue
		}
		module.SetAttributeTraversal(name, hcl.Traversal{
			hcl.TraverseRoot{Name: "var"}, hcl.TraverseAttr{Name: name},
		})
		variable := variables.Body().AppendNewBlock("variable", []string{name}).Body()
		variable.SetAttributeTraversal("type", hcl.Traversal{hcl.TraverseRoot{Name: "any"}})
		variable.SetAttributeValue("default", cty.EmptyObjectVal)
		variables.Body().AppendNewline()
	}
	return map[string][]byte{
		"main.tf": main.Bytes(), "variables.tf": variables.Bytes(),
		"providers.tf": providerFile(opts), "versions.tf": []byte(versionsTF),
		"README.md": []byte(readmeMD),
	}, nil
}

func providerFile(opts Options) []byte {
	file := hclwrite.NewEmptyFile()
	provider := file.Body().AppendNewBlock("provider", []string{"databricks"}).Body()
	provider.SetAttributeValue("host", cty.StringVal(opts.Host))
	if opts.Profile != "" {
		provider.SetAttributeValue("profile", cty.StringVal(opts.Profile))
	}
	return file.Bytes()
}

const versionsTF = `terraform {
  required_version = ">= 1.7.0"

  required_providers {
    databricks = {
      source  = "databricks/databricks"
      version = ">= 1.128.0, < 2.0.0"
    }
  }
}
`

const readmeMD = `# Exported Databricks root

This root manages only the configuration listed in export-report.json.
DataTF does not copy stored data or secret values.

The provider uses the exported workspace URL and selected profile.
Keep the profile available when you run Terraform. Never put credentials in this root.

## Import

Use a separate state for each root. Configure a remote backend for production.
Use one shared root for each metastore. Do not import the same object into multiple states.

1. Run terraform fmt -check -recursive.
2. Run terraform init.
3. Run terraform validate.
4. Run terraform plan -out=tfplan.
5. Run terraform show tfplan.
6. Confirm that the plan contains imports and no creates, updates, replacements, or deletes.
7. Run terraform apply tfplan after approval.
8. Delete imports.tf.

Do not apply a partial export or a plan with resource changes.
Save the state and reviewed plan before an ownership migration.

After the import, edit terraform.tfvars and review each plan.
Run later exports into a new directory to compare configurations.
`
