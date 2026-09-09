#!/usr/bin/env bash
# Local end-to-end check: export the fake workspace, import into local state,
# and require a no-change plan. Init needs network access for dependencies.
#
#   TF_BIN=tofu scripts/e2e-fake.sh [module-source]
set -euo pipefail

export DATATF_TELEMETRY=0

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
MODULES="${1:-$ROOT/../terraform-databricks-workspace}"
TF_BIN="${TF_BIN:-terraform}"
ADDR="${E2E_ADDR:-127.0.0.1:8787}"
KEEP="${E2E_KEEP:-0}"
CREATED_WORK=0
if [ -n "${E2E_WORK:-}" ]; then
  WORK="$E2E_WORK"
  mkdir -p "$WORK"
else
  WORK="$(mktemp -d)"
  CREATED_WORK=1
fi
cleanup() {
  if [ -n "${FAKE_PID:-}" ]; then
    kill "$FAKE_PID" 2>/dev/null || true
    wait "$FAKE_PID" 2>/dev/null || true
  fi
  if [ "$KEEP" = "1" ] || [ "$CREATED_WORK" = "0" ]; then
    echo "kept $WORK"
    return
  fi
  rm -r "$WORK"
}
trap cleanup EXIT

cd "$ROOT"
go build -o "$WORK/fakewsd" ./internal/fakews/cmd/fakewsd
"$WORK/fakewsd" -addr "$ADDR" >"$WORK/fakewsd.log" 2>&1 &
FAKE_PID=$!
sleep 1
kill -0 "$FAKE_PID"

go build -o "$WORK/datatf" ./cmd/datatf

export DATABRICKS_HOST="http://$ADDR" DATABRICKS_TOKEN=fake DATABRICKS_AUTH_TYPE=pat
export DATABRICKS_CONFIG_FILE="$WORK/none.cfg"
export DATABRICKS_DISCOVERY_URL="http://$ADDR/.well-known/oauth-authorization-server"
unset DATABRICKS_CONFIG_PROFILE || true

read -r -a layouts <<<"${E2E_LAYOUTS:-workspace resources}"
for layout in "${layouts[@]}"; do
  for scope in workspace shared; do
    OUT="$WORK/$layout/$scope"
    module_args=(--module-layout "$layout")
    if [[ "$layout" == workspace ]]; then
      module_args+=(--module-source "$MODULES")
    fi
    "$WORK/datatf" export --quiet --scope "$scope" --out "$OUT" \
      --scaffold "${module_args[@]}"
    (
      cd "$OUT"
      "$TF_BIN" fmt -check -recursive
      "$TF_BIN" init -input=false -no-color >/dev/null
      "$TF_BIN" validate -no-color
      # Mock tests check configuration; the real provider checks imports below.
      mv imports.tf "$WORK/$layout-$scope-imports.tf"
      mkdir tests
      cp "$ROOT/tests/export.tftest.hcl" tests/
      "$TF_BIN" test -no-color
      mv "$WORK/$layout-$scope-imports.tf" imports.tf
      set +e
      "$TF_BIN" plan -input=false -no-color -detailed-exitcode -out=tfplan \
        >plan-create.txt 2>plan.err
      code=$?
      set -e
      if [ "$code" -eq 1 ]; then
        echo "$TF_BIN plan failed for $scope:"
        cat plan.err
        KEEP=1
        exit 1
      fi
      "$TF_BIN" show -no-color tfplan >plan.txt
      grep -E "Plan: .*to import" plan.txt || {
        echo "no import summary in plan ($scope)"
        tail -40 plan.txt
        exit 1
      }
      if ! grep -qE "Plan: [1-9][0-9]* to import, 0 to add, 0 to change, 0 to destroy" plan.txt; then
        echo "plan for $scope is not import-only (exit $code):"
        grep -nE "will be|must be" plan.txt | grep -v "will be imported" || true
        KEEP=1
        exit 1
      fi
      echo "OK: $layout/$scope -> $(grep -E '^Plan:' plan.txt)"
      "$TF_BIN" apply -input=false -no-color tfplan >apply.txt
      mv imports.tf "$WORK/$layout-$scope-imports.tf"
      if ! "$TF_BIN" plan -input=false -no-color -detailed-exitcode >plan-after.txt 2>&1; then
        echo "$TF_BIN plan after import is not clean ($scope):"
        cat plan-after.txt
        KEEP=1
        exit 1
      fi
      echo "OK: $layout/$scope -> no changes after import"
    )
  done
done
