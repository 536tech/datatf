# GENERATED Terraform import blocks by datatf from https://adb-1111.1.azuredatabricks.net.
# Built from the same selection as the tfvars, so imports and tfvars match.
# Keep this file in the root ONLY for the import run: terraform plan, terraform apply,
# then delete it. Grants/permissions blocks are emitted only when the matching
# access map is populated, mirroring the modules' count-gating.

import {
  for_each = {
    shared_ref = "shared_ref"
  }
  to = module.workspace.module.catalog[each.key].databricks_catalog.this
  id = each.value
}

import {
  for_each = {
    shared_ref = "catalog/shared_ref"
  }
  to = module.workspace.module.catalog[each.key].databricks_grants.this[0]
  id = each.value
}

import {
  for_each = {
    "shared_ref.ref" = "shared_ref.ref"
  }
  to = module.workspace.module.schema[each.key].databricks_schema.this
  id = each.value
}

import {
  for_each = {
    shared_cred = "shared_cred"
  }
  to = module.workspace.module.storage_credential[each.key].databricks_storage_credential.this
  id = each.value
}

import {
  for_each = {
    "1111|storage_credential|shared_cred" = "1111|storage_credential|shared_cred"
    "2222|storage_credential|shared_cred" = "2222|storage_credential|shared_cred"
  }
  to = module.workspace.module.workspace_binding[each.key].databricks_workspace_binding.this
  id = each.value
}

import {
  for_each = {
    shared_cred = "storage_credential/shared_cred"
  }
  to = module.workspace.module.storage_credential[each.key].databricks_grants.this[0]
  id = each.value
}

import {
  for_each = {
    public_ref = "public_ref"
  }
  to = module.workspace.module.external_location[each.key].databricks_external_location.this
  id = each.value
}
