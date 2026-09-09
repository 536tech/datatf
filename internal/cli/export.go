package cli

import (
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/spf13/cobra"

	"github.com/536tech/datatf/internal/contract"
	"github.com/536tech/datatf/internal/emit"
	"github.com/536tech/datatf/internal/inventory"
	"github.com/536tech/datatf/internal/scaffold"
)

type exportOptions struct {
	scope        string
	outDir       string
	rootModule   string
	allowPartial bool
	scaffold     bool
	moduleSource string
	moduleVer    string
	moduleLayout string
	resources    []string
	name         *string
	profile      string
}

func newExportCommand(rc *runtime) *cobra.Command {
	opts := &exportOptions{}
	cmd := &cobra.Command{
		Use:   "export",
		Args:  usageArgs(cobra.NoArgs),
		Short: "Export supported platform configuration to Terraform",
		Long: `export writes terraform.tfvars, imports.tf, export.json, and
export-report.json for the supported module contract.

Use --scaffold to add the Terraform root files.
Use --module-layout resources for individual Registry modules instead of the workspace pattern.
Use --resources to read only the selected resource groups:
  ` + strings.Join(inventory.ResourceNames, "\n  ") + `

Use --name with one group to select one exact, case-sensitive name.
For service principals, use the key from inventory --json.

Catalogs include their schemas, grants, and workspace bindings.
Other groups include their supported permissions or grants.

--scope workspace (default) exports workspace resources and isolated Unity
Catalog objects bound only to this workspace.
--scope shared exports open Unity Catalog objects and isolated objects bound
to zero or many workspaces. It does not read workspace resource groups.

DataTF does not export cloud resources, account resources, workloads,
clusters, stored data, or secret values. Read failures block Terraform output
unless --allow-partial is set. A complete report covers only the selection
visible to your identity. Selection does not copy data or export dependencies
from other groups. Use a new output directory for each export.`,
		Example: `  datatf --profile analytics export --scaffold
  datatf --profile analytics export --resources catalogs --name sales --scaffold
  datatf --profile analytics export --resources warehouses --scaffold
  datatf --profile analytics export --resources warehouses --module-layout resources --scaffold
  datatf --profile analytics export --scope shared --resources catalogs --scaffold`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.run(cmd, rc)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.scope, "scope", "workspace", "workspace or shared")
	flags.StringVarP(&opts.outDir, "out", "o", "", "output directory (default datatf-<scope>)")
	flags.StringVar(&opts.rootModule, "root-module", "workspace",
		"composition module name; empty addresses per-resource modules directly")
	flags.BoolVar(&opts.allowPartial, "allow-partial", false,
		"write Terraform even when some reads failed")
	flags.BoolVar(&opts.scaffold, "scaffold", false, "also write a runnable Terraform root")
	flags.StringVar(&opts.moduleLayout, "module-layout", "workspace", "workspace or resources")
	flags.StringVar(&opts.moduleSource, "module-source", scaffold.DefaultModuleSource,
		"module source for --scaffold (registry address, Git URL, or local path)")
	flags.StringVar(&opts.moduleVer, "module-version", scaffold.DefaultModuleVersion,
		"Registry module version for --scaffold")
	flags.StringSliceVar(&opts.resources, "resources", nil,
		"limit reads to resource groups (comma-separated)")
	flags.String("name", "", "select one exact name within one --resources group")
	return cmd
}

func (opts *exportOptions) prepare(cmd *cobra.Command) error {
	if err := opts.prepareLayout(cmd); err != nil {
		return err
	}
	switch opts.scope {
	case "workspace", "shared":
	default:
		return fmt.Errorf("%w: --scope must be workspace or shared", errUsage)
	}
	if err := opts.validateRoot(); err != nil {
		return err
	}
	selected, name, err := resourceSelection(cmd, opts.resources, opts.scope == "shared")
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	opts.resources = selected
	opts.name = name
	if opts.outDir == "" {
		opts.outDir = "datatf-" + opts.scope
	}
	return nil
}

func (opts *exportOptions) validateRoot() error {
	if opts.rootModule != "" && !hclsyntax.ValidIdentifier(opts.rootModule) {
		return fmt.Errorf("%w: --root-module must be a valid Terraform identifier", errUsage)
	}
	if opts.scaffold && opts.moduleLayout == "workspace" && opts.rootModule == "" {
		return fmt.Errorf("%w: --scaffold needs a non-empty --root-module", errUsage)
	}
	return nil
}

func (opts *exportOptions) prepareLayout(cmd *cobra.Command) error {
	switch opts.moduleLayout {
	case "workspace":
		return nil
	case "resources":
	default:
		return fmt.Errorf("%w: --module-layout must be workspace or resources", errUsage)
	}
	if cmd.Flags().Changed("module-source") {
		return fmt.Errorf("%w: --module-source requires --module-layout workspace", errUsage)
	}
	if cmd.Flags().Changed("root-module") && opts.rootModule != "" {
		return fmt.Errorf("%w: --root-module must be empty with --module-layout resources", errUsage)
	}
	opts.rootModule = ""
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(opts.moduleVer) {
		return fmt.Errorf("%w: --module-version needs an exact release such as 1.0.0 "+
			"with --module-layout resources", errUsage)
	}
	return nil
}

func (opts *exportOptions) run(cmd *cobra.Command, rc *runtime) error {
	if err := opts.prepare(cmd); err != nil {
		return err
	}
	rc.usage.Scope, rc.usage.ResourceGroups = opts.scope, opts.resources
	ex, rep, err := opts.read(cmd, rc)
	if err != nil {
		return err
	}
	if rep.Status == "partial" {
		rc.usage.Outcome, rc.usage.ErrorCode = "partial", "partial_result"
	}
	if err := opts.checkReport(rc, rep); err != nil {
		return err
	}
	files, err := opts.files(ex, rep)
	if err != nil {
		return withHint(err, "render_error", "Check scaffold options with datatf export --help.")
	}
	written, err := emit.WriteFiles(opts.outDir, files)
	if err != nil {
		return outputError(err)
	}
	if err := writeExportSummary(rc, written, rep); err != nil {
		return err
	}
	opts.hintLayout(cmd, rc)
	return nil
}

func (opts *exportOptions) files(ex *contract.Export, rep *contract.Report) (
	map[string][]byte, error,
) {
	files, err := emit.ExportFiles(ex, rep, opts.rootModule)
	if err != nil {
		return nil, err
	}
	more, err := opts.scaffoldFiles(ex, rep)
	if err != nil {
		return nil, err
	}
	maps.Copy(files, more)
	return files, nil
}

func (opts *exportOptions) scaffoldFiles(ex *contract.Export, rep *contract.Report) (
	map[string][]byte, error,
) {
	options := scaffold.Options{
		Scope: ex.Scope, Host: rep.Host, Profile: opts.profile, RootModule: opts.rootModule,
		ModuleSource: opts.moduleSource, ModuleVersion: opts.moduleVer,
	}
	if opts.moduleLayout == "resources" {
		return scaffold.RenderResources(ex, options, opts.scaffold)
	}
	if opts.scaffold {
		return scaffold.Render(options)
	}
	return nil, nil
}

func (opts *exportOptions) read(cmd *cobra.Command, rc *runtime) (
	*contract.Export, *contract.Report, error,
) {
	ws, err := rc.client()
	if err != nil {
		return nil, nil, err
	}
	reader := inventory.New(ws)
	reader.Log, reader.Resources = rc.progress(), opts.resources
	reader.Name = opts.name
	inv, err := reader.Read(cmd.Context())
	if err != nil {
		return nil, nil, workspaceError(err)
	}
	ex, issues := contract.Build(inv, contract.Scope(opts.scope))
	opts.profile = ws.Config.Profile
	return ex, contract.NewReport(inv, ex, issues, "datatf "+version, now()), nil
}

func (opts *exportOptions) checkReport(rc *runtime, rep *contract.Report) error {
	if rep.Status == "complete" || opts.allowPartial {
		return nil
	}
	if rc.out.IsJSON() {
		if err := rc.out.JSON(rep); err != nil {
			return err
		}
	}
	return rc.partialError("export", rep.Issues)
}

const layoutHint = "Hint: this export selects one or more resource groups. " +
	"Use --module-layout resources to call the individual Registry modules. " +
	"See docs/module-layouts.md."

// needsLayoutHint reports whether a narrowed export in the workspace layout needs the hint.
func needsLayoutHint(narrowed bool, layout string) bool {
	return narrowed && layout == "workspace"
}

// hintLayout writes the resource layout hint to the progress channel after a written export.
func (opts *exportOptions) hintLayout(cmd *cobra.Command, rc *runtime) {
	narrowed := cmd.Flags().Changed("resources") || cmd.Flags().Changed("name")
	log := rc.progress()
	if log == nil || !needsLayoutHint(narrowed, opts.moduleLayout) {
		return
	}
	log("%s", layoutHint)
}

func writeExportSummary(rc *runtime, files []string, rep *contract.Report) error {
	if rc.out.IsJSON() {
		return rc.out.JSON(rep)
	}
	rc.out.Println()
	for _, name := range files {
		rc.out.Printf("wrote %s\n", name)
	}
	rc.out.Printf("status: %s, %d imports, %d excluded (other scope), %d skipped\n",
		rep.Status, rep.Imports, len(rep.Excluded), len(rep.Skipped))
	return nil
}
