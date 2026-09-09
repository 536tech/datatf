package scaffold

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/536tech/datatf/internal/contract"
)

func TestResourceRootDeterminismAndProfile(t *testing.T) {
	ex := &contract.Export{Scope: contract.ScopeWorkspace, Tfvars: &contract.Tfvars{
		Catalogs: map[string]contract.CatalogSettings{
			`b"${1+1}`: {IsolationMode: "ISOLATED", Owner: "owner"},
			"a":        {IsolationMode: "ISOLATED", Owner: "owner", Comment: "a catalog"},
		},
	}}
	opts := Options{Host: "https://workspace.example.com", Profile: `profile"${1+1}`,
		ModuleVersion: DefaultModuleVersion}
	before, err := json.Marshal(ex)
	if err != nil {
		t.Fatal(err)
	}
	files, err := RenderResources(ex, opts, true)
	if err != nil {
		t.Fatal(err)
	}
	assertResourceRenderStable(t, ex, opts, files)
	provider := parseBlock(t, files["providers.tf"]).Body
	assertAttribute(t, provider, "profile", opts.Profile)
	assertAttribute(t, provider, "host", opts.Host)
	after, err := json.Marshal(ex)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("rendering changed the canonical export")
	}
}

func assertResourceRenderStable(t *testing.T, ex *contract.Export, opts Options,
	want map[string][]byte,
) {
	t.Helper()
	for range 10 {
		got, err := RenderResources(ex, opts, true)
		if err != nil {
			t.Fatal(err)
		}
		for name, data := range want {
			if !bytes.Equal(data, got[name]) {
				t.Fatalf("%s is not deterministic", name)
			}
		}
	}
}

func TestResourceRootEmptyAndInputsOnly(t *testing.T) {
	ex := &contract.Export{Scope: contract.ScopeShared, Tfvars: &contract.Tfvars{}}
	opts := Options{ModuleVersion: DefaultModuleVersion}
	files, err := RenderResources(ex, opts, false)
	if err != nil || len(files) != 1 || files["terraform.tfvars"] == nil {
		t.Fatalf("inputs-only files: %v, %v", files, err)
	}
	files, err = RenderResources(ex, opts, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(files["main.tf"]) != 0 || len(files["variables.tf"]) != 0 {
		t.Fatal("empty selection emitted module blocks or variables")
	}
}
