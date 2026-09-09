# Export permissions

DataTF reads configuration and access rules. It does not change Databricks resources or read secret
values. Some read APIs require permissions that also allow changes. Use an existing approved identity.

## Required access

- **Secret scopes:** `MANAGE` on each scope is required to read its ACLs. `READ` is insufficient.
  The identity can also read and change secrets, although DataTF does neither. [Source][secrets]
- **Catalogs and schemas:** ownership or `USE CATALOG` / `USE SCHEMA` permits metadata access under
  the API rules. Metastore admins have broader visibility. Lists can omit inaccessible objects.
  These permissions alone do not establish access to all grants. [Catalogs][catalogs], [schemas][schemas]
- **Storage credentials and external locations:** the APIs permit metastore admins, owners, or
  callers with an object privilege. They do not name one minimum read privilege.
  [Credentials][credentials], [locations][locations]
- **Unity Catalog grants:** Databricks documents full grant visibility for metastore admins, object
  or parent catalog/schema owners, and principals with `READ METADATA` or `MANAGE`. Others see their own
  grants. This general rule still needs verification against DataTF's REST requests. [Source][grants]
- **Isolated workspace bindings:** the object owner or a metastore admin is required. DataTF needs
  bindings to select the Terraform root. `READ METADATA` alone is not confirmed. [Source][bindings]
- **Policies, pools, warehouses, and workspace service principals:** the minimum access for the full
  export is unconfirmed. `CAN VIEW` covers warehouse details, not a verified full ACL export.
  Confirm access with your workspace admin. [ACLs][acls], [permissions API][permissions], [SCIM][scim]

[`READ METADATA`][metadata] permits metadata and grant reads without data reads or object changes.
Follow its parent-usage requirements. It is not a verified permission set for a complete DataTF export.
Workspace admin access does not replace the Unity Catalog requirements above.

## Before you apply

1. Confirm the profile and workspace with `datatf auth status --profile <profile>`.
2. Use `--resources` to select only the groups you need.
3. Confirm the exported objects and grants with their owners.
4. Require an imports-only Terraform plan before apply.

Use `--name` with one group to limit object detail reads to one exact name.
DataTF still needs list access. Name selection does not grant access or verify complete grant visibility.

`status: complete` means no reported read or build errors—not full visibility.
APIs can return filtered objects, grants, or metadata without an error.
A shared export does not prove full metastore coverage.
A failed read blocks Terraform output unless you use `--allow-partial`.
`--allow-partial` does not restore missing access. Keep partial output for review only.

Terraform import and apply need separate provider and state permissions.
Incomplete access rules can cause unwanted changes: the provider manages [grants][tf-grants] and
[ACLs][tf-permissions] authoritatively.

Sources checked September 6, 2026. These are documented rules, not a live permission test.

[secrets]: https://docs.databricks.com/api/secrets/v1/list-acls
[catalogs]: https://docs.databricks.com/api/uc-catalogs/v1/list-catalogs
[schemas]: https://docs.databricks.com/api/uc-schemas/v1/list-schemas
[credentials]: https://docs.databricks.com/api/uc-credentials/v1/get-storage-credential
[locations]: https://docs.databricks.com/api/uc-external-locations/v1/list-external-locations
[grants]: https://learn.microsoft.com/en-us/azure/databricks/data-governance/unity-catalog/manage-privileges/
[bindings]: https://docs.databricks.com/api/uc-workspace-bindings/v1/get-workspace-bindings
[acls]: https://learn.microsoft.com/en-us/azure/databricks/security/auth/access-control/
[permissions]: https://docs.databricks.com/api/access-management/v1/get-object-permissions
[scim]: https://docs.databricks.com/api/scim/v1/list-service-principals
[metadata]: https://learn.microsoft.com/en-us/azure/databricks/data-governance/unity-catalog/access-control/privileges-reference#read-metadata
[tf-grants]: https://github.com/databricks/terraform-provider-databricks/blob/main/docs/resources/grants.md
[tf-permissions]: https://github.com/databricks/terraform-provider-databricks/blob/main/docs/resources/permissions.md
