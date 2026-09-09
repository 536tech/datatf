# GENERATED Terraform import blocks by datatf from https://adb-1111.1.azuredatabricks.net.
# Built from the same selection as the tfvars, so imports and tfvars match.
# Keep this file in the root ONLY for the import run: terraform plan, terraform apply,
# then delete it. Grants/permissions blocks are emitted only when the matching
# access map is populated, mirroring the modules' count-gating.

import {
  for_each = {
    sales = "sales"
  }
  to = module.workspace.module.catalog[each.key].databricks_catalog.this
  id = each.value
}

import {
  for_each = {
    "1111|catalog|sales"                = "1111|catalog|sales"
    "1111|external_location|lake_raw"   = "1111|external_location|lake_raw"
    "1111|storage_credential|lake_cred" = "1111|storage_credential|lake_cred"
  }
  to = module.workspace.module.workspace_binding[each.key].databricks_workspace_binding.this
  id = each.value
}

import {
  for_each = {
    sales = "catalog/sales"
  }
  to = module.workspace.module.catalog[each.key].databricks_grants.this[0]
  id = each.value
}

import {
  for_each = {
    "sales.bronze" = "sales.bronze"
    "sales.silver" = "sales.silver"
  }
  to = module.workspace.module.schema[each.key].databricks_schema.this
  id = each.value
}

import {
  for_each = {
    "sales.bronze" = "schema/sales.bronze"
  }
  to = module.workspace.module.schema[each.key].databricks_grants.this[0]
  id = each.value
}

import {
  for_each = {
    lake_cred = "lake_cred"
  }
  to = module.workspace.module.storage_credential[each.key].databricks_storage_credential.this
  id = each.value
}

import {
  for_each = {
    lake_cred = "storage_credential/lake_cred"
  }
  to = module.workspace.module.storage_credential[each.key].databricks_grants.this[0]
  id = each.value
}

import {
  for_each = {
    lake_raw = "lake_raw"
  }
  to = module.workspace.module.external_location[each.key].databricks_external_location.this
  id = each.value
}

import {
  for_each = {
    lake_raw = "external_location/lake_raw"
  }
  to = module.workspace.module.external_location[each.key].databricks_grants.this[0]
  id = each.value
}

import {
  for_each = {
    "Job Family Policy" = "P3"
    "Team Policy"       = "P1"
  }
  to = module.workspace.module.cluster_policy[each.key].databricks_cluster_policy.this
  id = each.value
}

import {
  for_each = {
    "Job Family Policy" = "/cluster-policies/P3"
    "Team Policy"       = "/cluster-policies/P1"
  }
  to = module.workspace.module.cluster_policy[each.key].databricks_permissions.this[0]
  id = each.value
}

import {
  for_each = {
    shared-pool = "pool-1"
  }
  to = module.workspace.module.instance_pool[each.key].databricks_instance_pool.this
  id = each.value
}

import {
  for_each = {
    "Analytics WH" = "wh1"
  }
  to = module.workspace.module.warehouse[each.key].databricks_sql_endpoint.this
  id = each.value
}

import {
  for_each = {
    "Analytics WH" = "/sql/warehouses/wh1"
  }
  to = module.workspace.module.warehouse[each.key].databricks_permissions.this[0]
  id = each.value
}

import {
  for_each = {
    db-scope = "db-scope"
    kv-scope = "kv-scope"
  }
  to = module.workspace.module.secret_scope[each.key].databricks_secret_scope.this
  id = each.value
}

import {
  for_each = {
    "db-scope|||jon@example.com" = {
      id    = "db-scope|||jon@example.com"
      index = "jon@example.com"
      key   = "db-scope"
    }
    "kv-scope|||admins" = {
      id    = "kv-scope|||admins"
      index = "admins"
      key   = "kv-scope"
    }
    "kv-scope|||data-engineers" = {
      id    = "kv-scope|||data-engineers"
      index = "data-engineers"
      key   = "kv-scope"
    }
  }
  to = module.workspace.module.secret_scope[each.value.key].databricks_secret_acl.this[each.value.index]
  id = each.value.id
}

import {
  for_each = {
    "dup-sp (a1b2c3d4-0000-0000-0000-000000000002)" = "101"
    "dup-sp (a1b2c3d4-0000-0000-0000-000000000003)" = "102"
    etl-sp                                          = "100"
  }
  to = module.workspace.module.service_principal[each.key].databricks_service_principal.this
  id = each.value
}
