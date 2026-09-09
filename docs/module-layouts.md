# Module layouts

DataTF supports two Terraform root layouts.
Both use the same resource selection and ownership rules.
Neither creates Azure workspaces or exports workloads, stored data, or secret values.

| `--module-layout` | Root calls | Default module version |
| --- | --- | --- |
| `workspace` (default) | `536tech/workspace/databricks` | `1.0.0` |
| `resources` | Individual `536tech` Registry modules | `1.0.0` |

## Individual modules

Export one warehouse into a new root:

```sh
datatf export --profile analytics --resources warehouses --name "Analytics WH" \
  --module-layout resources --scaffold --out ./warehouse
```

Omit `--resources` and `--name` to export all supported objects visible in the selected scope.
Use `--scope shared` for shared Unity Catalog objects. Export shared objects once per metastore.

DataTF writes module blocks, variables, provider settings, versions, inputs, and matching import blocks.
It emits only module types with selected objects. Each type uses `for_each` with
the existing object keys. The root uses one state, not one state per resource module.

| Module label and input | Registry module |
| --- | --- |
| `catalog` | `536tech/catalog/databricks` |
| `schema` | `536tech/schema/databricks` |
| `storage_credential` | `536tech/storage-credential/databricks` |
| `external_location` | `536tech/external-location/databricks` |
| `workspace_binding` | `536tech/workspace-binding/databricks` |
| `cluster_policy` | `536tech/cluster-policy/databricks` |
| `instance_pool` | `536tech/instance-pool/databricks` |
| `warehouse` | `536tech/sql-warehouse/databricks` |
| `secret_scope` | `536tech/secret-scope/databricks` |
| `service_principal` | `536tech/service-principal/databricks` |

The `warehouse` label stays consistent with the import addresses.
Its Registry name is `sql-warehouse`.
The modules manage the resources in the [resource matrix](../README.md#resource-coverage).

## Versions and inputs

The resource layout pins every emitted module to an exact version. Use `--module-version 1.0.0`
to select that release explicitly. A different version must exist for every selected module and keep
the same inputs and resource addresses. Version ranges are not supported for this layout.

Generated sources include the `registry.terraform.io` hostname.
This selects the same module Registry in Terraform and OpenTofu.
See the [OpenTofu example](../examples/opentofu/README.md).

`--module-source` applies only to the workspace layout.
It does not adapt an arbitrary module interface.
The resource layout has no outer module label, so it rejects a nonempty `--root-module` override.

Resource-layout inputs use the module labels above as keys in `terraform.tfvars`.
Each object contains its module inputs, including its name and supported access rules.
Schema keys use `catalog.schema`. Unity Catalog grants use lists of principals and privileges.
`export.json` retains the existing canonical data structure for tools that read it.

Without `--scaffold`, DataTF writes inputs, imports, and JSON files only. Use `--scaffold` for a
complete root. References outside the selected resource groups must already exist.
See [resource selection](resource-selection.md) for details.

## State safety

Choose the layout before the first import. Changing the layout changes Terraform addresses.
Do not replace an existing root with the other layout or import its objects into another state.
An existing state needs a separately reviewed migration. DataTF does not migrate state.

For each new root:

```sh
terraform fmt -check -recursive
terraform init
terraform validate
terraform plan -out=tfplan
terraform show tfplan
```

Require imports only: no creates, updates, replacements, or deletes. Apply the saved plan only after
approval. Remove `imports.tf` after import.
Run `terraform plan -detailed-exitcode` and require exit 0.

Use one workspace state per workspace and one shared state per metastore.
Configure a separate remote backend key for each production root.
Save the existing state and reviewed plan before any migration.
