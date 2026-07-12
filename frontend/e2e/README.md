# End-to-end tests (Playwright)

These specs drive the real app in a browser against a **running instance**.

## Run

1. Bring up the stack (from the repo root):

   ```bash
   make up            # Postgres + MinIO + the app on http://localhost:8080
   ```

   The app starts in development mode, so the specs can use the dev-login form.

2. Install browsers once, then run the tests (from `frontend/`):

   ```bash
   npm install
   npx playwright install chromium
   npm run test:e2e
   ```

Point the tests at a different instance with `PLAYWRIGHT_BASE_URL`:

```bash
PLAYWRIGHT_BASE_URL=https://topology.example.org npm run test:e2e
```

## What's covered

- `auth.spec.ts` — unauthenticated redirect to `/login`; dev-login reaches the
  dashboard; role-based sidebar (admin sees Backup/Audit, plain users don't).
- `proposal.spec.ts` — register-a-resource creates a pending proposal via the
  UI; server-side JSON Schema validation rejects an invalid payload (no FQDN).

## Notes

- Tests that create proposals leave draft/pending rows; run against a disposable
  dev database (the compose stack) rather than production data.
- To also test approval → apply, seed a resource group first
  (`import-tree`/`import-github`) so the applied resource has a group to attach
  to.
