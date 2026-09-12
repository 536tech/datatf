package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

// RenderVariable renders one tfvars block as formatted HCL. Values pass
// through JSON so the same shape feeds export.json, which keeps the two
// outputs in lockstep.
func RenderVariable(v Variable, header []string) ([]byte, error) {
	return RenderVariables([]Variable{v}, header)
}

// RenderVariables renders populated variables into one formatted tfvars file.
func RenderVariables(variables []Variable, header []string) ([]byte, error) {
	var buf bytes.Buffer
	writeComments(&buf, header)
	if len(header) > 0 {
		buf.WriteString("\n")
	}
	for i, variable := range variables {
		val, err := toCty(variable.Value)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", variable.Name, err)
		}
		file := hclwrite.NewEmptyFile()
		file.Body().SetAttributeValue(variable.Name, val)
		writeComments(&buf, variable.Comment)
		buf.Write(hclwrite.Format(file.Bytes()))
		if i < len(variables)-1 {
			buf.WriteString("\n")
		}
	}
	return buf.Bytes(), nil
}

// RenderImports renders Terraform import blocks. rootModule is the name of the
// composition module instance in the target root ("workspace" yields
// module.workspace.module.catalog["x"]...); empty addresses the per-resource
// modules directly.
func RenderImports(imports []Import, rootModule string, header []string) []byte {
	file := hclwrite.NewEmptyFile()
	body := file.Body()
	groups := groupImports(imports)
	for i, group := range groups {
		block := body.AppendNewBlock("import", nil)
		block.Body().SetAttributeValue("for_each", importValues(group))
		block.Body().SetAttributeRaw("to", importTargetTokens(group.key, rootModule))
		block.Body().SetAttributeTraversal("id", importIDTraversal(group.key))
		if i < len(groups)-1 {
			body.AppendNewline()
		}
	}
	var buf bytes.Buffer
	writeComments(&buf, header)
	if len(header) > 0 {
		buf.WriteString("\n")
	}
	buf.Write(hclwrite.Format(file.Bytes()))
	return buf.Bytes()
}

type importGroupKey struct {
	module    string
	resource  string
	indexKind string
	index     int
}

type importGroup struct {
	key     importGroupKey
	imports []Import
}

func groupImports(imports []Import) []importGroup {
	groups := make([]importGroup, 0, len(imports))
	positions := make(map[importGroupKey]int)
	for _, imp := range imports {
		key := importKey(imp)
		position, ok := positions[key]
		if !ok {
			position = len(groups)
			positions[key] = position
			groups = append(groups, importGroup{key: key})
		}
		groups[position].imports = append(groups[position].imports, imp)
	}
	return groups
}

func importKey(imp Import) importGroupKey {
	key := importGroupKey{module: imp.Module, resource: imp.Resource}
	switch index := imp.Index.(type) {
	case int:
		key.indexKind = "number"
		key.index = index
	case string:
		key.indexKind = "string"
	}
	return key
}

func importValues(group importGroup) cty.Value {
	if group.key.indexKind != "string" {
		values := make(map[string]cty.Value, len(group.imports))
		for _, imp := range group.imports {
			values[imp.Key] = cty.StringVal(imp.ID)
		}
		return cty.MapVal(values)
	}
	values := make(map[string]cty.Value, len(group.imports))
	for _, imp := range group.imports {
		attributes := map[string]cty.Value{
			"id":  cty.StringVal(imp.ID),
			"key": cty.StringVal(imp.Key),
		}
		if index, ok := imp.Index.(string); ok {
			attributes["index"] = cty.StringVal(index)
		}
		values[imp.ID] = cty.ObjectVal(attributes)
	}
	return cty.MapVal(values)
}

func importTargetTokens(group importGroupKey, rootModule string) hclwrite.Tokens {
	t := hcl.Traversal{hcl.TraverseRoot{Name: "module"}}
	if rootModule != "" {
		t = append(t, hcl.TraverseAttr{Name: rootModule}, hcl.TraverseAttr{Name: "module"})
	}
	t = append(t, hcl.TraverseAttr{Name: group.module})
	key := eachTraversal("key")
	if group.indexKind == "string" {
		key = valueTraversal("key")
	}
	tokens := appendTraversalIndex(hclwrite.TokensForTraversal(t), key)
	tokens = append(tokens,
		&hclwrite.Token{Type: hclsyntax.TokenDot, Bytes: []byte(".")},
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte(group.resource)},
		&hclwrite.Token{Type: hclsyntax.TokenDot, Bytes: []byte(".")},
		&hclwrite.Token{Type: hclsyntax.TokenIdent, Bytes: []byte("this")},
	)
	switch group.indexKind {
	case "number":
		tokens = appendTraversalIndex(tokens, hclwrite.TokensForValue(cty.NumberIntVal(int64(group.index))))
	case "string":
		tokens = appendTraversalIndex(tokens, valueTraversal("index"))
	}
	return tokens
}

func importIDTraversal(group importGroupKey) hcl.Traversal {
	traversal := hcl.Traversal{
		hcl.TraverseRoot{Name: "each"},
		hcl.TraverseAttr{Name: "value"},
	}
	if group.indexKind == "string" {
		traversal = append(traversal, hcl.TraverseAttr{Name: "id"})
	}
	return traversal
}

func eachTraversal(attribute string) hclwrite.Tokens {
	return hclwrite.TokensForTraversal(hcl.Traversal{
		hcl.TraverseRoot{Name: "each"},
		hcl.TraverseAttr{Name: attribute},
	})
}

func valueTraversal(attribute string) hclwrite.Tokens {
	traversal := hcl.Traversal{hcl.TraverseRoot{Name: "each"}, hcl.TraverseAttr{Name: "value"}}
	return hclwrite.TokensForTraversal(append(traversal, hcl.TraverseAttr{Name: attribute}))
}

func appendTraversalIndex(tokens, index hclwrite.Tokens) hclwrite.Tokens {
	tokens = append(tokens, &hclwrite.Token{Type: hclsyntax.TokenOBrack, Bytes: []byte("[")})
	tokens = append(tokens, index...)
	return append(tokens, &hclwrite.Token{Type: hclsyntax.TokenCBrack, Bytes: []byte("]")})
}

// commentLine is the only text that bypasses hclwrite. A line break would end the comment,
// so it is never allowed through.
var commentLine = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\u2028", " ", "\u2029", " ")

func writeComments(buf *bytes.Buffer, lines []string) {
	for _, line := range lines {
		buf.WriteString("# " + commentLine.Replace(line) + "\n")
	}
}

// toCty converts any JSON-marshalable value into a cty value. Objects become
// cty objects with sorted attributes, arrays become tuples, and numbers keep
// their exact textual form.
func toCty(v any) (cty.Value, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return cty.NilVal, err
	}
	var value ctyjson.SimpleJSONValue
	if err := value.UnmarshalJSON(raw); err != nil {
		return cty.NilVal, err
	}
	return value.Value, nil
}

// Header returns the generated-file banner for tfvars output.
func Header(scope Scope, host string) []string {
	rule := "Unity Catalog includes only ISOLATED securables bound exclusively to this workspace."
	if scope == ScopeShared {
		rule = "Unity Catalog includes OPEN securables and ISOLATED securables bound to zero or many workspaces."
	}
	return []string{
		"GENERATED by datatf from " + strings.TrimRight(host, "/") + " (scope: " + string(scope) + ").",
		rule,
		"Review before importing.",
	}
}

// ImportsHeader returns the banner for imports.tf.
func ImportsHeader(host string) []string {
	return []string{
		"GENERATED Terraform import blocks by datatf from " + strings.TrimRight(host, "/") + ".",
		"Built from the same selection as the tfvars, so imports and tfvars match.",
		"Keep this file in the root ONLY for the import run: terraform plan, terraform apply,",
		"then delete it. Grants/permissions blocks are emitted only when the matching",
		"access map is populated, mirroring the modules' count-gating.",
	}
}
