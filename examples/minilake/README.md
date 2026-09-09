# Local MiniLake demo

Run DataTF against [MiniLake](https://github.com/dmux/minilake) in Docker.
The demo uses the real Databricks CLI, DataTF binary, workspace module, and Terraform provider.
It imports one SQL warehouse into one local state. It does not contact a cloud Databricks workspace.

## Run

Use Bash on macOS, Linux, or WSL.
Install Docker with Compose, Go, Terraform 1.7 or later, the Databricks CLI, curl, and jq.
Select a local Docker context. The script rejects remote Docker endpoints.
Start Docker. From the DataTF repository, run:

```sh
make demo
```

Terraform downloads the pinned workspace module from GitHub.
If you already have a compatible module checkout, use it instead:

```sh
bash scripts/demo-minilake.sh /absolute/path/to/terraform-databricks-workspace
```

Each run resets the disposable `datatf-demo` container and removes its previous local objects.
The script keeps exports, plans, and state under `.demo/`. Git ignores that directory.
The script applies only after it verifies a plan with one import and no resource changes.
It then requires a second plan with no changes.

MiniLake stays available at `https://127.0.0.1:18443`. To use the same local profile:

```sh
export DATABRICKS_CONFIG_FILE="$PWD/examples/minilake/databrickscfg"
databricks auth profiles
./bin/datatf auth status --profile minilake
```

Run `make build` first if `bin/datatf` does not exist.
The demo profile contains a dummy token. It does not change your normal Databricks configuration.
Certificate checks are disabled for this loopback profile only. Never use that setting for a remote workspace.

Stop the demo:

```sh
docker compose -p datatf-demo -f examples/minilake/compose.yaml down
```

## Coverage and limits

The image pins MiniLake 1.7.4 and applies `compatibility.patch` during the build.
The patch preserves warehouse fields and supplies host and warehouse connection metadata.
Without those fields, the provider rejects the configuration or plans an update during import.
This is a patched MiniLake test, not proof that the unmodified image supports DataTF.

The test does not verify real authentication, permissions, OAuth, or ODBC connections.
It does not cover Unity Catalog grants, bindings, storage credentials, or external locations.
MiniLake does not implement those APIs. DataTF still reports selected API failures as errors.

Use `make e2e` for both scopes and the full resource contract through the fake workspace.
Use a real Databricks workspace to verify cloud access and provider behavior.
