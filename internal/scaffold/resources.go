package scaffold

import (
	"maps"
	"slices"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"

	"github.com/536tech/datatf/internal/contract"
)

// RenderResources renders module inputs and, when requested, their runnable root.
func RenderResources(ex *contract.Export, opts Options, root bool) (map[string][]byte, error) {
	modules, err := contract.ResourceModules(ex.Tfvars)
	if err != nil {
		return nil, err
	}
	inputs := make([]contract.Variable, 0, len(modules))
	for _, module := range modules {
		inputs = append(inputs, contract.Variable{
			Name: module.Name, Value: ctyjson.SimpleJSONValue{Value: module.Values},
		})
	}
	tfvars, err := contract.RenderVariables(inputs, contract.Header(ex.Scope, opts.Host))
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{"terraform.tfvars": tfvars}
	if !root {
		return files, nil
	}
	main, variables := resourceRoot(modules, opts.ModuleVersion)
	maps.Copy(files, map[string][]byte{
		"main.tf": main, "variables.tf": variables,
		"providers.tf": providerFile(opts), "versions.tf": []byte(versionsTF),
		"README.md": []byte(readmeMD + resourceReadmeMD),
	})
	return files, nil
}

const resourceReadmeMD = `
## Individual resource modules

This root calls individual Registry modules, without an outer workspace module.
The module blocks pin their versions. terraform.tfvars uses the module labels as input names.
export.json retains the canonical DataTF data structure, not the resource-module input shape.

Keep this layout after import. A layout change requires a reviewed state migration.
`

func resourceRoot(modules []contract.ResourceModule, version string) ([]byte, []byte) {
	main, variables := hclwrite.NewEmptyFile(), hclwrite.NewEmptyFile()
	for _, module := range modules {
		body := main.Body().AppendNewBlock("module", []string{module.Name}).Body()
		body.SetAttributeValue("source", cty.StringVal(module.Source))
		body.SetAttributeValue("version", cty.StringVal(version))
		body.SetAttributeTraversal("for_each", traversal("var", module.Name))
		for _, name := range resourceInputNames(module.Values) {
			body.SetAttributeRaw(name, hclwrite.TokensForFunctionCall("try",
				hclwrite.TokensForTraversal(traversal("each", "value", name)),
				hclwrite.TokensForValue(cty.NullVal(cty.DynamicPseudoType)),
			))
		}
		resourceDependencies(body, module.Name, modules)
		main.Body().AppendNewline()
		variable := variables.Body().AppendNewBlock("variable", []string{module.Name}).Body()
		variable.SetAttributeValue("description", cty.StringVal("Inputs for "+module.Source+"."))
		// Resource objects can have different policy JSON shapes; the child validates its inputs.
		variable.SetAttributeTraversal("type", traversal("any"))
		variable.SetAttributeValue("default", cty.EmptyObjectVal)
		variables.Body().AppendNewline()
	}
	return main.Bytes(), variables.Bytes()
}

func resourceInputNames(values cty.Value) []string {
	names := map[string]bool{}
	for _, value := range values.AsValueMap() {
		for name := range value.AsValueMap() {
			names[name] = true
		}
	}
	return slices.Sorted(maps.Keys(names))
}

func resourceDependencies(body *hclwrite.Body, name string, modules []contract.ResourceModule) {
	dependencies := map[string][]string{
		"schema":            {"catalog"},
		"external_location": {"storage_credential"},
		"workspace_binding": {"catalog", "storage_credential", "external_location"},
	}
	tokens := []hclwrite.Tokens{}
	for _, module := range modules {
		if slices.Contains(dependencies[name], module.Name) {
			tokens = append(tokens, hclwrite.TokensForTraversal(traversal("module", module.Name)))
		}
	}
	if len(tokens) > 0 {
		body.SetAttributeRaw("depends_on", hclwrite.TokensForTuple(tokens))
	}
}

func traversal(root string, attributes ...string) hcl.Traversal {
	value := hcl.Traversal{hcl.TraverseRoot{Name: root}}
	for _, name := range attributes {
		value = append(value, hcl.TraverseAttr{Name: name})
	}
	return value
}
