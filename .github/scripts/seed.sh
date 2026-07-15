#!/usr/bin/env bash
# Seed a minimal dataset into a freshly-started stack so the e2e suite has
# something to work with. Everything here is best-effort: the specs use
# test.skip() when prerequisites are missing, so a partial seed degrades
# coverage rather than failing the run.
set -euo pipefail

BASE="${PLAYWRIGHT_BASE_URL:-http://localhost:8080}"
JAR="$(mktemp)"

echo "Waiting for $BASE to come up…"
for i in $(seq 1 60); do
  if curl -fsS -o /dev/null "$BASE/healthz" 2>/dev/null; then
    echo "App is up."
    break
  fi
  sleep 2
  if [ "$i" = 60 ]; then echo "App never became healthy" >&2; exit 1; fi
done

api() { curl -fsS -b "$JAR" -c "$JAR" "$@"; }

echo "Logging in (dev admin)…"
api -X POST "$BASE/api/v1/auth/dev-login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"ci-admin@example.org","role":"administrator"}' >/dev/null

echo "Syncing institutions from the registry (best-effort)…"
SYNCED=$(api -X POST "$BASE/api/v1/admin/institutions/sync" 2>/dev/null \
  | python3 -c 'import sys,json;print(json.load(sys.stdin).get("synced",0))' 2>/dev/null || echo 0)
echo "Institutions synced: $SYNCED"

if [ "${SYNCED:-0}" -gt 0 ]; then
  IID=$(api "$BASE/api/v1/institutions?q=a" \
    | python3 -c 'import sys,json;d=json.load(sys.stdin);print(d[0]["id"] if d else "")')
  if [ -n "$IID" ]; then
    echo "Creating a facility/site/resource-group via an atomic bundle…"
    BODY=$(IID="$IID" python3 - <<'PY'
import json, os
iid = os.environ["IID"]
print(json.dumps({
  "entity_kind": "bundle", "operation": "create", "submit": True,
  "proposed_state": {"operations": [
    {"entity_kind": "facility", "operation": "create",
     "proposed_state": {"name": "CI_Facility", "institution_id": iid}},
    {"entity_kind": "site", "operation": "create",
     "proposed_state": {"name": "CI_Site", "facility": "CI_Facility", "long_name": "CI Site"}},
    {"entity_kind": "resource_group", "operation": "create",
     "proposed_state": {"name": "CI_ResourceGroup", "site": "CI_Site"}},
  ]},
}))
PY
)
    PID=$(api -X POST "$BASE/api/v1/proposals" -H 'Content-Type: application/json' -d "$BODY" \
      | python3 -c 'import sys,json;print(json.load(sys.stdin)["id"])')
    api -X POST "$BASE/api/v1/proposals/$PID/approve" >/dev/null
    echo "Seeded resource group CI_ResourceGroup."
  fi
else
  echo "No institutions available — facility/RG-dependent specs will skip."
fi

echo "Seed complete."
