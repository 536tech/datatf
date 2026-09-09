# OpenTofu

Use DataTF exports with OpenTofu, the Databricks provider, and individual Registry modules.
Install [OpenTofu](https://opentofu.org/docs/intro/install/) alongside DataTF.
Keep Git on your `PATH`; the published module downloads from GitHub.
No Terraform binary, adapter, or extra DataTF package is required.
`tofu init` downloads the module and provider.

## Export your workspace

Read the [authentication and permission steps](../../README.md#quick-start) before an export.
Select a saved Databricks profile whose URL matches your workspace. This example uses `analytics`:

```sh
databricks auth profiles
datatf auth status --profile analytics
datatf export --profile analytics --out ./export --scaffold \
  --module-layout resources
cd export
tofu fmt -check -recursive
tofu init
tofu validate
tofu plan -out=tfplan
tofu show tfplan
```

The resource layout names `registry.terraform.io` explicitly. OpenTofu resolves an unqualified
[module source](https://opentofu.org/docs/language/modules/sources/#module-registry)
through its own registry. Each generated module block pins version `1.0.0`.
The generated root requires version 1.7 or later.

Require imports only, with no creates, updates, replacements, or deletes.
After approval, run `tofu apply tfplan`. Remove `imports.tf`, then require a no-change `tofu plan`.
Use a remote backend for production.
Do not point Terraform and OpenTofu at separate states for the same objects.
An existing Terraform state needs a separate migration review.

## Local test

The local test also needs Go and Bash. Run this command from the DataTF source checkout:

```sh
TF_BIN=tofu E2E_KEEP=1 E2E_LAYOUTS=resources bash scripts/e2e-fake.sh
```

The test exports workspace and shared scopes from the local fake workspace.
This includes Unity Catalog fixtures.
It requires imports-only plans, applies them to local test states, and requires no-change plans.
It prints the directory with the exports, states, and plans.
Network access is required for downloads.

No Docker, Databricks credentials, or cloud resources are required.
This test checks module and provider compatibility. It does not test real Databricks permissions.

The default workspace pattern uses a separate module interface. Its `1.0.0` release references
child modules without a Registry hostname. This example uses the resource layout instead.
Keep an existing state on its current layout until you review a migration.
