# Select resources

Use `--resources` for resource groups. Use `--name` for one object within one group.
Both `inventory` and `export` accept these options.
The [resource matrix](../README.md#resource-coverage) lists the group names.

## Select a group or object

Export all visible SQL warehouses:

```sh
datatf export --profile analytics --resources warehouses --out ./warehouses --scaffold
```

Export one warehouse:

```sh
datatf export --profile analytics --resources warehouses --name "Analytics WH" --out ./wh --scaffold
```

Select multiple groups with a comma-separated list, such as `--resources catalogs,warehouses`.
Omit both options to use the default groups for the selected scope.

Names are exact and case-sensitive. Quote names that contain spaces.
`--name` requires one resource group. It does not accept patterns or a list of names.
A missing or duplicate name makes the export partial and blocks Terraform output by default.
DataTF cannot distinguish an absent object from an object your identity cannot see.

To check names and ownership, inspect the inventory:

```sh
datatf inventory --profile analytics --resources catalogs --json
```

For service principals, use `key` from the inventory instead of the display name.
Duplicate display names have an identifier suffix in that key.
Selection preserves the same key and import address as an export of the whole group.

## Selection and module layout

`--resources` and `--name` choose the objects. `--module-layout` chooses the modules and the
Terraform addresses. A narrower selection does not change the layout.
The default `workspace` layout calls the `536tech/workspace/databricks` pattern module.
Use `--module-layout resources` to call the individual `536tech` Registry modules directly:

```sh
datatf export --profile analytics --resources catalogs --module-layout resources --scaffold
```

A selected catalog keeps its schemas in both layouts.
See [module layouts](module-layouts.md) for the module table, the versions, and the state rules.

## Catalogs and dependencies

A selected catalog includes its supported schemas, direct grants, and workspace bindings.
Schemas, grants, ACLs, and bindings are not separate selectors.
Other groups include their supported permissions or grants.
Selection does not read secret values or copy data.

DataTF still lists the selected group to find the object.
It filters names before it reads object details, children, and permissions.
The [export permissions](permissions.md) still apply.

Selection does not bypass ownership rules or include dependencies from other groups.
For example, an external location retains a reference to its existing storage credential.
It does not add that credential to the export.
The generated root assumes that these external dependencies still exist.

Use `--scope shared` for an object that belongs to the shared scope:

```sh
datatf export --profile analytics --scope shared --resources catalogs --name shared_ref \
  --out ./shared-catalog --scaffold
```

## Review before import

Check `resources`, `name`, `counts`, `issues`, `skipped`, and `excluded` in the report.
DataTF omits `name` when you do not select one object.
A complete report means no read or build issues for the visible selection, not full workspace coverage.
An object in the other scope appears under `excluded` and produces no imports.
System or unsupported objects appear under `skipped`.

Use a new directory for each export. A selected export is not an update to an existing Terraform root.
Replacing a full root with a subset can make Terraform propose removal of omitted objects.
Keep each object in one state and under one managing tool.
DataTF does not detect another state or Bundle that already manages the object.

Review an imports-only plan before any apply. Keep the existing state and the reviewed plan.
Selection does not require a new state for each object.
For an existing root, review and merge the selected configuration and imports into its intended owner.
