# datatf

Export existing Azure Databricks platform configuration into Terraform files, ready for import.
DataTF reads your workspace and writes local files. It never reads secret values or applies Terraform.

## Install

macOS and Linux:

```sh
curl -fsSL https://datatf.io/install.sh | sh
```

Windows PowerShell:

```powershell
irm https://datatf.io/install.ps1 | iex
```

The script verifies the release checksum and installs one binary. See the
[install guide](docs/install.md) for manual downloads, macOS Gatekeeper, and source builds.

## Quick start

If you already have a profile, skip sign-in and replace `analytics` with its name.

1. Install the [Databricks CLI](https://learn.microsoft.com/en-us/azure/databricks/dev-tools/cli/install).
2. Check the [required read permissions](docs/permissions.md).
3. Sign in to your Azure Databricks workspace. Replace `<workspace-url>` with its URL:

```sh
databricks auth login --host "<workspace-url>" --profile analytics
```

4. Confirm the profile URL, then export into a new directory:

```sh
databricks auth profiles
datatf auth status --profile analytics
datatf export --profile analytics --out ./export --scaffold
```

Open `export/README.md` for the Terraform import steps.
Review the export report and Terraform plan before you apply it. Require imports only, with no resource changes.

The default export covers workspace-owned resources.
For shared Unity Catalog objects, see [scopes and state ownership](docs/module-layouts.md#state-safety).

## Resource coverage

Only the Terraform resources in this table are supported.

| `--resources` group | Terraform resources | Scope |
| --- | --- | --- |
| `catalogs` | `databricks_catalog`, `databricks_schema`, `databricks_grants` | Workspace or shared |
| `storage_credentials` | `databricks_storage_credential`, `databricks_grants` | Workspace or shared |
| `external_locations` | `databricks_external_location`, `databricks_grants` | Workspace or shared |
| Included with the three groups above | `databricks_workspace_binding` | Workspace or shared |
| `cluster_policies` | `databricks_cluster_policy`, `databricks_permissions` | Workspace |
| `instance_pools` | `databricks_instance_pool`, `databricks_permissions` | Workspace |
| `warehouses` | `databricks_sql_endpoint`, `databricks_permissions` | Workspace |
| `secret_scopes` | `databricks_secret_scope`, `databricks_secret_acl` | Workspace |
| `service_principals` | `databricks_service_principal` | Workspace |

DataTF does not export Azure infrastructure, account or metastore setup, workloads, stored data, or secret values.

## Documentation

- [Select resources](docs/resource-selection.md) or [choose a module layout](docs/module-layouts.md).
- [Troubleshooting](docs/troubleshooting.md), [upgrades](docs/updates.md), and [telemetry](docs/telemetry.md).
- [Contributing](CONTRIBUTING.md) and [changelog](CHANGELOG.md).

## License

[Apache-2.0](LICENSE).
