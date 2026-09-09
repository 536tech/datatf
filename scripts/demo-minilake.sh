#!/usr/bin/env bash
set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
EXAMPLE="$REPO/examples/minilake"
COMPOSE=(docker compose -p datatf-demo -f "$EXAMPLE/compose.yaml")

for tool in docker go terraform databricks curl jq; do
  command -v "$tool" >/dev/null || {
    echo "Install $tool first." >&2
    exit 1
  }
done

DOCKER_ENDPOINT="${DOCKER_HOST:-$(docker context inspect --format '{{.Endpoints.docker.Host}}')}"
if [ -n "${DOCKER_CONTEXT:-}" ]; then
  DOCKER_ENDPOINT="$(docker context inspect "$DOCKER_CONTEXT" --format '{{.Endpoints.docker.Host}}')"
fi
case "$DOCKER_ENDPOINT" in
unix://* | npipe://*) ;;
*)
  echo "Select a local Docker context before this demo. Remote Docker endpoints are not supported." >&2
  exit 1
  ;;
esac

# Do not inherit cloud credentials, profiles, or Terraform overrides from the caller.
local_command() {
  env -i PATH="$PATH" HOME="$HOME" \
    DATABRICKS_CONFIG_FILE="$EXAMPLE/databrickscfg" \
    DATABRICKS_CACHE_DIR="$WORK/cache" GIT_TERMINAL_PROMPT=0 \
    TF_IN_AUTOMATION=1 CHECKPOINT_DISABLE=1 DATATF_TELEMETRY=0 "$@"
}

echo "Reset the disposable datatf-demo container. Previous local objects will be removed."
"${COMPOSE[@]}" down
"${COMPOSE[@]}" up --build -d --wait --wait-timeout 120

mkdir -p "$REPO/.demo"
WORK="$(mktemp -d "$REPO/.demo/minilake.XXXXXX")"
echo "Keep the export, state, and plans in $WORK"

curl --fail --silent --show-error --insecure \
  https://127.0.0.1:18443/api/2.0/sql/warehouses \
  -H 'Content-Type: application/json' --data-binary "@$EXAMPLE/warehouse.json" \
  >"$WORK/created.json"
jq -e '.id | length > 0' "$WORK/created.json" >/dev/null

cd "$REPO"
go build -trimpath -o "$WORK/datatf" ./cmd/datatf
local_command databricks auth profiles
local_command "$WORK/datatf" auth status --profile minilake --json
local_command "$WORK/datatf" inventory --profile minilake --resources warehouses --json \
  >"$WORK/inventory.json"

EXPORT_ARGS=(export --profile minilake --resources warehouses --scaffold --out "$WORK/root")
if [ "$#" -gt 0 ]; then
  EXPORT_ARGS+=(--module-source "$(cd "$1" && pwd)")
fi
local_command "$WORK/datatf" "${EXPORT_ARGS[@]}"
jq -e '.status == "complete" and .resources == ["warehouses"] and .imports == 1' \
  "$WORK/root/export-report.json" >/dev/null

local_command terraform -chdir="$WORK/root" fmt -check -recursive
local_command terraform -chdir="$WORK/root" init -input=false -no-color
local_command terraform -chdir="$WORK/root" validate -no-color
local_command terraform -chdir="$WORK/root" plan -input=false -no-color -out=tfplan
local_command terraform -chdir="$WORK/root" show -json tfplan >"$WORK/plan.json"
jq -e '
  ([.resource_changes[]? | select(.change.importing != null)] | length) == 1
  and all(.resource_changes[]?; .change.actions == ["no-op"])
' "$WORK/plan.json" >/dev/null

echo "The reviewed plan imports one warehouse and makes no resource changes."
local_command terraform -chdir="$WORK/root" apply -input=false -no-color tfplan
mv "$WORK/root/imports.tf" "$WORK/reviewed-imports.tf"
local_command terraform -chdir="$WORK/root" plan -input=false -no-color -detailed-exitcode \
  -out=after-import.tfplan

echo "PASS: profile auth, export, one import, and a no-change plan."
echo "Evidence: $WORK"
echo "MiniLake stays available at https://127.0.0.1:18443."
echo "Stop it with: docker compose -p datatf-demo -f examples/minilake/compose.yaml down"
