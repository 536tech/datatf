# datatf agent notes

Go CLI that exports supported platform configuration from an existing Azure Databricks workspace
into an opinionated Terraform root.

## Layout

- `cmd/datatf` - binary entry point
- `internal/cli` - Cobra commands: auth status, inventory, export, version, telemetry
- `internal/telemetry` - consent controls and allowlisted command metrics
- `internal/inventory` - reads the workspace into a typed model; ownership rules
- `internal/contract` - tfvars model, builder, import blocks, HCL render, report
- `internal/emit` - writes files
- `internal/scaffold` - `--scaffold` root files
- `internal/fakews` - httptest fake workspace + fixtures for tests
- `.agents/skills/datatf` - canonical agent workflow for the CLI
- `.goreleaser.yaml` - macOS, Linux, and Windows GitHub release archives

## Rules

- Read-only against Databricks. Never read secret values.
- Set `DATATF_TELEMETRY=0` for agent tasks unless the user explicitly approves telemetry.
- Before a telemetry change, read `docs/telemetry.md`. Keep workspace metadata outside events.
- Before an interactive read, run `databricks auth profiles` and match the profile to the target URL.
- Pass the selected `--profile` value to each DataTF command.
- Before an export or access recommendation, read `docs/permissions.md`.
- A complete report does not certify full object or grant visibility. Confirm coverage with the owners.
- The resource matrix in `README.md` is the product boundary.
- Keep Azure, account, metastore bootstrap, and workload resources outside the current contract.
- Classify supported Unity Catalog objects as workspace, shared, or system.
- tfvars and imports are built from one selection in `contract.Build`; never
  emit one without the other.
- Any read failure makes the export partial; partial never writes Terraform
  without `--allow-partial`.
- `resources` and optional `name` define report coverage. Complete does not mean all workspace objects.
- Render the full output before writes. Preserve existing files, including symlinks.
- HCL goes through `hclwrite` + `cty`; never string-build HCL.
- Read `.agents/skills/datatf/SKILL.md` when an agent uses or extends the CLI.
- Add a resource type to the inventory, module contract, fake workspace, matrix, and tests together.
- Golden files: `go test ./internal/contract -run TestGolden -update`, then
  read the diff before committing.
- New API calls need a fixture route in `internal/fakews/testdata/routes.json`.

## Checks

`make check` must pass. `make e2e` (needs terraform + network for the provider)
exports the fake workspace, scaffolds a root against
`../terraform-databricks-workspace`, and asserts `terraform plan` is
import-only for both scopes. Run it before any change to the contract or the
modules. Workflows: `zizmor .github/workflows/*.yml`.

For a local Docker demo, read `examples/minilake/README.md` and run `make demo`.
The MiniLake test covers a SQL warehouse. It does not replace the full contract or cloud tests.
