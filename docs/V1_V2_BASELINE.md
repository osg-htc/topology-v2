# v1 → v2 Baseline

Working understanding of where this project came from, what it is now, and what
still has to change. Complements `SPEC_CONFORMANCE_AUDIT.md` (2026-08-10), which
is the gap list; this doc is the *map* plus the findings that audit doesn't carry.

## The four artifacts

| Thing | Where | What it is |
|---|---|---|
| **v1** | `~/Coding/topology` | The live system. Python 3 / Flask, ~11k LOC, data = YAML in git, changes arrive as GitHub PRs. 15,267 commits, HEAD 2026-08-05. Still merging daily. |
| **v2** | `~/Coding/topology-v2` (this repo) | The rewrite. Go 1.26 + chi + Postgres + S3, Next.js SPA embedded via `go:embed`. ~10k Go + ~5.8k TS. 34 commits, 2026-07-11→07-15, built as labeled "Phases 0–6". |
| **The spec** | `~/Coding/docs/feature_set_v1.md` | Black-box behavioral spec of v1, written to drive the rewrite. Byte-identical to `topology-v7.md` — the 7th draft (earlier: `topology.md`, `topology-entity-first.md`, `topology-feature-set.md`, `topology-v2..v6.md`). `topology-analysis.md` is the companion tech-stack recommendation. |
| **The audit** | `docs/SPEC_CONFORMANCE_AUDIT.md` | v2-vs-spec conformance pass, plus a v1 re-verification pass and a live side-by-side run. Its §5 self-corrects 21 spec overstatements — read that section, it matters. |

Note: the audit cites `docs/FEATURE_SET.md`, which is **not in this repo** and never
was. The spec lives outside the repo at the path above.

## v1 architecture (the contract we're matched against)

**Storage.** YAML in git; entity names come from directory/file names, not `Name:` fields.

```
topology/services.yaml                        # service name -> numeric id (60 entries)
topology/support-centers.yaml                 # 52 SCs; Contacts here are type -> LIST (not rank-keyed)
topology/<Facility>/FACILITY.yaml             # 230 (only keys ever present: ID, InstitutionID)
topology/<Facility>/<Site>/SITE.yaml          # 405; open-ended bag — unknown keys are re-emitted sorted
topology/<Facility>/<Site>/<RG>.yaml          # 429; Resources nested inside
topology/<Facility>/<Site>/<RG>_downtime.yaml # 175; YAML list, "# ---" separators
virtual-organizations/<VO>.yaml               # 122
virtual-organizations/REPORTING_GROUPS.yaml   # global reporting-group table
projects/<Project>.yaml                       # 1556 (flat)
projects/_CAMPUS_GRIDS.yaml                   # campus-grid name -> int id (19)
mappings/{field_of_science,institution_ids,nsfscience,project_institution}.yaml
```

Exactly three levels deep, never more. **Contacts are not in this repo** — they
come from a private Bitbucket repo (`git@bitbucket.org:opensciencegrid/contact.git`,
`contacts.yaml`) merged with a live CILogon/COmanage LDAP query (LDAP wins on
conflict). API-key hashes live in a separate `API_KEYS_FILE`.

**Serving.** Flask + `xmltodict`, four WSGI entrypoints (prod × ITB, app × webhook).
Per-dataset `CachedData` caches; a daemon `Timer` thread refreshes topology at
`max(cache_lifetime * 2/3, 60)`s with ±10% jitter; VOs/projects/mappings/contacts
refresh lazily. `strict = config.get("STRICT", app.debug)` — strict in dev (errors
propagate), lenient in prod (log + serve stale).

**Auth.** Three ORed mechanisms: bearer API key (`tk-<uuid4>`, SHA-256 hashed,
**hard 401 with no fallback** on a bad token), GridSite client-cert DN via
`GRST_CRED_AURI_*` WSGI environ (**soft fail-through**), and a dev-only `AUTH`
bypass honored only when `app.debug`. Authorization unlocks exactly contact PII:
Email / Phone / SMSAddress / first DN inside `rgsummary`, `vosummary`, `miscuser`,
`/contacts`; and it is a hard 403 gate on `/oasis-managers/json`.

**Change review.** GitHub PR → `webhook_app.py` (HMAC-SHA1 `X-Hub-Signature`, 1 MiB
cap) → `automerge_check.py`. Auto-merge requires: head up to date with base, ≥1
changed file, **every** changed file matching `^topology/[^/]+/[^/]+/[^/]+_downtime.yaml$`,
the GitHub user resolving to a registered contact, and the submitter being a listed
contact on every affected resource. Diffing is keyed by downtime ID. A new project
`Organization` or an unrecognized `InstitutionID` is the one hard `REQUEST_CHANGES`.
Six outcome codes map to a message taxonomy: RC 0 approve+merge, RC 1/2/3/4/5
comment (5 blocks), **RC 6 deliberately silent**.

**Validation.** 5 GitHub workflows. `validate-data` runs XSD validation of all four
feeds, downtime rules (4 accepted time formats, ID uniqueness via a text grep),
`verify_resources.py` (18 active checks), the site-name regex `^[A-Za-z0-9_ -]+$`
applied to the site *directory* name, `verify_projects.py` (ResourceAllocations
references only), and an authfile smoke test. Two more workflows check facility
`InstitutionID` and — on a daily cron — project field-of-science precision against
an OSG Elasticsearch usage query.

**UI.** Flask/Jinja2 + Bootstrap + grid.js + lunr.js, all public except `/contacts`
(cert-gated). Homepage sitemap, Leaflet map (`iframe.html.j2`, 1641 lines), searchable
Resources/Collaborations tables with a documented lunr micro-syntax, localStorage
query history, a 576px table↔card breakpoint, three guided-authoring wizards.

## v2 architecture (what we have)

**Storage.** Postgres is the source of truth. 10 goose migrations, 22 tables, UUID
PKs, soft-delete + partial unique indexes on active rows. The YAML tree is
**import/export only** — a backup/restore bridge until switchover.

Present as tables: facilities, sites, resource_groups, resources, resource_services,
resource_contacts, downtimes, services, support_centers, institutions (live-synced
cache), vos, projects, entity_contacts (a v2 invention: contacts on RG/site/facility),
contact_replacements, plus auth/proposal/audit tables.

Absent entirely: reporting groups, namespaces/DataFederations, all VO substructure
(`vos` is `name` + `vo_id` + `disable` + one opaque `raw_yaml` TEXT blob), the three
mappings reference tables, campus grids, contact DNs, and all contact PII beyond an
encrypted email.

**Serving.** ~10 legacy-compatible routes + a `/api/v1/*` tree for the SPA. All five
XSDs are byte-identical to v1's and served at `/schema/{file}`. No Cache-Control
anywhere. CORS is global `*` (v1 is strictly per-route).

**Auth.** OIDC (CILogon) session cookie only. Roles `administrator | manager | user`
plus an orthogonal `contact_reader` capability. `OptionalAuth` is on exactly two
routes and unlocks exactly two fields (`CILogonID`, decrypted `Email`).

**Change flow.** Propose → revise → approve, 100% human-gated. 7 proposable kinds,
JSON-Schema-validated at submit, all business rules deferred to apply time. Bundled
atomic multi-entity proposals. Invites, contact-replacement hand-offs, email
verification, audit log, S3 + GitHub backup/restore.

## The gap, by kind

**1 — Whole subsystems absent.** OSDF/StashCache credential generation (every
`/cache/*`, `/origin/*`, `/stashcache/*`, `/osdf/*` route, plus the 8 CLI tools);
bearer-token and client-cert auth; the automated change-review policy engine;
`/metrics`; the public read-only UI (homepage, map, organizations, collaborations,
contacts, resource-files); `/rgdowntime/ical`, `/miscuser/xml`,
`/api/resource_group_summary`, `/nsfscience/csv`, `/institution_ids`,
`/api/institutions`, `/collaborations/osg-scitokens-mapfile.conf`,
`/oasis-managers/json`, `/resources/stashcache-files`; the Cache-Control contract;
the plaintext `400 "Invalid arguments: …"` filter-error contract; the second
(test/ITB) environment.

Unmatched paths do not 404 — they `301 → ./` into the SPA shell, and `/map/iframe`
and `/stashcache/namespaces` actually infinite-redirect.

**2 — Present but wrong.** The audit's §2 table is the reference. The two with the
largest measured blast radius:

- `Resource.Active` default is **inverted** (`nil` → false; v1 defaults true) — 18 real resources flip.
- `Resource.Disable` is fabricated as `!Active` instead of being an independent field — **338** real resources get a wrong value in `/rgsummary/xml`, the most-consumed feed.
- `downtime_attrs_showpast` is a no-op, so the default `/rgdowntime/xml` returns 3,776 entries where v1 returns 15.

**3 — Architecturally different by design.** Postgres vs git; propose-approve vs
GitHub PRs; internal authenticated console vs public site. These are choices, not
bugs — but they're why the "swap the internals and no consumer notices" test fails.

## Findings not in the existing audit

### ★ The OSDF namespace registry is being sunset out of topology

Git-verified in v1:

- `0026b0cb6` **2026-03-11** "Removing all the DataFederations information" — 498 deletions across 14 VO files (CMS, ET, Fermilab, Gluex, HCC, IceCube, JLab, LIGO, MOLLER, NSG, PATh, Sage, VDC, XENON).
- `796bbe8fc` same day: LIGO's config added back per a design doc.
- `9a73fdfd2` **2026-05-05**: "**Temporarily** restore GlueX OSDF info for **legacy StashCache** services."

Only **LIGO** and **Gluex** carry `DataFederations.StashCache` today (11 namespaces).
Compounding this: `get_namespaces_info` is **XRootD-only** — it ignores Pelican
caches/origins — and all 12 XRootD caches in current data are `Active: false`. So
`/osdf/namespaces` today returns `"caches": []` with default filters.

Namespace registration has moved to Pelican's own registry. **This materially
changes how much of the OSDF subsystem v2 actually needs** — likely a compatibility
shim for two VOs' legacy StashCache, not the full pipeline. Confirm before sizing.

### Real-data usage counts (prioritization signal)

Resources — ~1100 total: Services 1100, ContactLists 1100, VOOwnership 603,
WLCGInformation 221, Tags 201 (CC* 121, OSPool 82), DN 147, AllowedVOs 141,
FQDNAliases 53. Service instances: CE 407, Submit Node 155, Squid 119,
Pelican cache 51, Pelican origin 50, XRootD origin 32, XRootD component 22,
XRootD cache 12 → **~145 cache/origin resources** still poll the missing endpoints.

Projects — 1556: InstitutionID and FieldOfScienceID on essentially all (1 literal
`Unknown`), Sponsor.CampusGrid 963 vs Sponsor.VirtualOrganization 76,
**ResourceAllocations only 6**.

VOs — 122: Contacts 116, FieldsOfScience 113, OASIS 110, **ReportingGroups 103**,
ParentVO 41, Credentials/TokenIssuers 16. So the unimplemented `ReportingGroups`
expansion affects 103 of 122 VOs and the missing `ParentVO.ID` affects 41 — both a
direct consequence of storing VOs as one opaque blob.

### v2 correctness issues beyond the audit's list

**Proposal workflow**
1. `transition()` (`internal/handlers/proposals.go:164-201`) enforces **no source-status precondition**. A creator can re-`submit` an `applied` proposal back to `pending`, making it re-approvable and re-appliable — a replay.
2. Approve's status update sits **outside** the apply transaction (`:229-239`) → a crash in that window leaves an applied-but-still-`pending` proposal, i.e. double-apply.
3. `base_version` is written and read back but **never compared to anything** — there is no conflict detection at all. It also stores only `{resource_id, name}`, not field values (`:678-685`).
4. Business rules run only at apply time, so reviewers routinely approve proposals that then 400. `facility.institution_id` is optional in the JSON schema but mandatory at apply — a guaranteed reviewer-visible failure mode.
5. Resource "update" is soft-delete + insert; if the payload omits `ID`, the persisted `topology_id` flips from the explicit legacy value to `GenID(name)` and disappears from exported YAML.

**Auth / security**
6. Anonymous `POST /api/v1/auth/dev-login` mints an **administrator** session whenever `APP_ENV != production` — and `development` is the default. The caller-supplied role string is inserted with no enum check.
7. The OIDC ID-token signature is never verified; no `nonce`/`aud`/`exp` validation (deliberate, with a comment — the token comes from the token endpoint over TLS — but worth knowing).
8. `role_claim` invite acceptance **seizes a contact slot with no approval step**, and any authenticated user can mint such an invite.
9. `MarkInviteUsed` lacks `WHERE used_at IS NULL` → single-use is enforced only by a read-then-write check, so concurrent accepts can both pass.
10. Email-verification confirm matches on `token_hash` alone, not scoped to the session user.
11. `/vosummary/json` returns the **raw VO YAML**, publishing 40-hex contact-ID hashes and OASIS manager DNs unauthenticated — while v1 deliberately strips the raw ID and substitutes `CILogonID`, and v2's own `/rgsummary/xml` carefully gates the same identifiers.
12. `/rgsummary/xml` for a `contact_reader` carries decrypted email but sets neither `private` nor `no-store`, so a shared cache in front could serve PII to anonymous clients.
13. `GET /api/v1/contacts` gates on `HasContactReader` but is registered **without** `OptionalAuth`, so contact IDs are blanked unconditionally — even for administrators.
14. The master key is silently auto-generated in production if `INSTANCE_KEY` is unset. No rotation or re-wrap path exists.
15. Email delivery is a **stub** — in production the verification token is written to the application log and never sent. No SMTP dependency in `go.mod`.

**Round-trip / data loss**
16. `downtimes` has no `extra` column, so `Downtime.Extra` is silently dropped through the DB — the one place the package's "inline `Extra` guarantees lossless round-trip" promise is broken by the schema.
17. `virtual-organizations/REPORTING_GROUPS.yaml` is imported **as a VO** (`ReadVOs` has no name filter, unlike `ReadProjects`), so a bogus VO named `REPORTING_GROUPS` appears in `/vosummary/xml` and the counts.
18. `projects/_CAMPUS_GRIDS.yaml` is skipped on import and never re-emitted on export → real backup/restore data loss.
19. `entity_contacts` (contacts on RG/site/facility) has no YAML representation and is not exported → contacts added on a parent entity vanish from a backup.
20. `downtimes.dt_id` is not unique yet is the update/delete key, and creation mints `DtID: now.Unix()` — two downtimes created in the same second collide and one edit rewrites both.
21. `UpsertService` does `ON CONFLICT (name) DO UPDATE SET id = $1` — it mutates the primary key.
22. Restore/import is `TRUNCATE … CASCADE` then import, **not in a transaction**; it also wipes `contact_replacements` (workflow state, not topology data).
23. The GitHub integration is **inbound only** — one tarball GET. There is no commit, push, or PR anywhere, so "export back to the GitHub repo" does not exist.

Also: the `api_keys` table exists in migration `002_auth.sql` with a full schema and
**zero Go references** — a ready-made home if bearer auth gets rebuilt.

### Corrections / additions to the spec and audit

- The spec's "≥48h notice = scheduled" advisory: v1's actual threshold is **24 hours**, the note is **bidirectional**, and it is non-blocking. (Audit correction #20 caught the number; the bidirectionality is additional.)
- The "3 individuals granted via explicit FQANs" carve-out is not only test-fixture-only (audit #18) — it is now **dead**. No personal DNs remain in any namespace; the last was removed in `3332a6484` (2026-03-04). The underlying leak (no-FQDN + `AllowedCaches: [ANY]` exposes every DN) is structural and would recur.
- v1's canonical downtime output format `"%b %d, %Y %H:%M %p %Z"` uses 24-hour `%H` *and* appends `%p`, so it really emits `"Sep 08, 2021 14:50 PM UTC"`. Reproducing v1 faithfully means reproducing that.
- v1 has one unparseable data file: `projects/Merrimack_Mahata.yaml:7` has a literal TAB after `InstitutionID:`, so PyYAML skips the whole project and it never reaches `/miscproject/xml`.

### v1 bugs worth deciding to *fix* rather than mirror

- `data_federation.py:364` does `self.errors += {…}` on a `set` → `TypeError`; any malformed `CredentialGeneration` silently drops the **entire VO**.
- `_parse_authz_dict` returns `None` for `{}` → `TypeError` on unpack → same VO drop. A multi-key authz dict silently keeps only the first key.
- `/cache/grid-mapfile` and `/origin/grid-mapfile` use `assert stashcache` instead of the `503` guard the other 7 endpoints use → uncontrolled 500 (and a *different* failure under `python -O`).
- The `for/else` in `_get_resource_with_services` raises on the **first** non-matching service name, so `[XROOTD_CACHE_SERVER, PELICAN_CACHE]` never reaches the second entry — Pelican-only caches/origins are rejected by the FQDN resolver.
- `ResourceMissingServices` maps to 404 on the Authfile endpoints and 400 on scitokens/grid-mapfile — inconsistent.
- `/github/login` is an **open redirect**: `redirect_uri` comes from the query string (and originally from an unvalidated `Referer`) and is passed through `urlparse(...).geturl()`, which is a no-op round-trip, not validation. Only `state` is checked. The spec's "restricted to a known allowlist" describes the fix, not v1.
- With no `WEBHOOK_SECRET_KEY`, `validate_webhook_signature` returns `None` and the caller treats that as **pass** — the webhook is fully unauthenticated.
- The webhook fires only on `pull_request` action `opened`, so pushing a fix to a rejected PR never re-evaluates eligibility.
- `resource_contact_ids` KeyErrors on a resource with no `ContactLists`; `check_resource_contacts` reads the RG at the raw `BASE_SHA` rather than the resolved base.
- Four network dependencies gate CI/merge: live `topology.opensciencegrid.org/miscuser/xml` (twice) and `topology-institutions.osg-htc.org` (twice). The FOS-precision check additionally hits an OSG Elasticsearch cluster on a daily cron, so it can go red with no repo change.
- The Mapbox access token is committed in `iframe.html.j2:1279`.

## Settled scope (answered 2026-08-18)

**The mandate is a carbon copy.** v2 must present the same shape to an outside
consumer as v1, and must **output at minimum every field v1 emits**. Routes cut
over one at a time until all have moved, with the goal that no consumer ever
notices. Nothing that works today may break. Bug fixes and improvements come
*later*, as a separate phase.

The stated bar is *shape and field completeness*, not literal byte equality — see
the caveat under consequence 3.

Consequences that reframe everything above:

1. **v1's bugs are requirements.** The "v1 bugs worth deciding to fix rather than
   mirror" section is now a **reproduce** list, not a fix list: the `+=`-on-a-set
   VO drop, `assert stashcache` → 500 on the two grid-mapfile routes, the
   `for/else` that rejects Pelican-only resources, `ResourceMissingServices`
   mapping to 404 on Authfile endpoints but 400 elsewhere, the open redirect in
   `/github/login`, the webhook passing with no secret configured, and the
   `"Sep 08, 2021 14:50 PM UTC"` 24-hour-plus-`%p` time format.
2. **Where the spec and v1 disagree, v1 wins.** `SPEC_CONFORMANCE_AUDIT.md` §5's
   21 corrections become the authoritative reading rather than caveats — the
   24-hour (not 48-hour) downtime advisory, the unenforced ContactType vocab, the
   non-required Site fields, the nonexistent `endpoint_override`, and the rest.
   The spec is a map; v1's running code is the territory.
3. **Formatting-level divergence is an open risk, not a stated requirement.**
   The user asked for the same shape and at minimum the same fields; they
   explicitly did *not* ask for byte-for-byte output. So the following are things
   to **raise and get a ruling on**, not assume: `xml.Header` emitting uppercase
   `UTF-8` vs v1's lowercase `utf-8`; 2-space indent vs v1's tabs; element ordering
   within repeated siblings; float formatting (`4.8e+06` vs `4800000`);
   authfile ID emission order (insertion vs sorted); v1's
   `json.dumps(sort_keys=True)` on every JSON feed; the trailing space in the dummy
   scitokens block's `audience = `.

   These matter because some consumers do string comparison or regex matching
   rather than XML/JSON parsing — v1's own `test_api.py` asserts several legacy/new
   endpoint pairs are byte-identical, and the grid-mapfile tests match on an exact
   line regex. But which of these are genuinely load-bearing is a per-consumer
   question we have not answered. Do not silently expand scope here.
4. **Differential testing is the primary tool.** v1 runs locally against the same
   YAML checkout, so v1's actual output is ground truth. A harness that boots both
   and diffs output across every route × filter combination is worth more than code
   comparison, and should become a permanent CI gate. The audit's §6 did this ad
   hoc; it needs to be systematic. v1's own tests will not carry us — e.g. the
   OSDF fixtures are thin and its `verify_origin_authfile.sh` is broken and unwired.
5. **OSDF gets reproduced as-is**, including `/osdf/namespaces` currently returning
   `"caches": []`, the XRootD-only scoping that excludes Pelican, and the two
   remaining VOs. The sunset finding no longer argues for building less — but it
   still tells us the path is low-traffic with thin fixtures, so we must generate
   our own comparison corpus rather than trusting v1's test suite.
6. **Auth reproduces v1's three mechanisms exactly**, including hard-401-no-fallback
   for bearer and soft-fail-through for client-cert DN. Note the deployment
   dependency: v1 reads `GRST_CRED_AURI_*` WSGI environ set by GridSite/Apache in
   front of it. v2 terminates no TLS today (`ListenAndServe`, no `TLSConfig`), so
   parity needs that front-end component too, not just Go code.
7. **The UI is the sanctioned exception** — it can and will change, as a secondary
   goal after feed parity.
8. **Second environment: not now.** v2 currently holds cloned v1 data, so the whole
   instance is effectively a test environment. No production-data risk, which means
   destructive re-imports and schema churn are cheap right now.

### Two things the mandate does not resolve on its own

- **The contact corpus.** v1's contact data comes from a private Bitbucket repo
  merged with a live CILogon/COmanage LDAP query. For `/miscuser/xml` and the
  auth-gated fields inside `/rgsummary/xml` and `/vosummary/xml` to carry the same
  fields v1 emits, v2 needs that same corpus *and* the fields it doesn't model:
  `SecondaryEmail`, `PrimaryPhone`, `SecondaryPhone`, `IM`, `SMSAddress`, `DNs`,
  `ContactPreference`, `Flags`, `GitHub`, `Profile`, `PhotoURL`, the
  md5-of-email `GravatarURL`, and the dedup-by-`CILogonID` rule. This is the
  largest *data-model* consequence of the carbon-copy mandate — not merely a
  missing route.
- **The SPA catch-all is dangerous during cutover.** v2 answers every unmatched
  path with `301 → ./` into the app shell. During a route-by-route migration an
  un-migrated path must behave like v1 (a real 404, or a proxy pass-through to
  v1), never silently swallow the request. `/map/iframe` and
  `/stashcache/namespaces` currently infinite-redirect.

### One thing parity fixes for free

v2's `/vosummary/json` returns the raw VO YAML, publishing 40-hex contact-ID
hashes and OASIS manager DNs unauthenticated — v1 deliberately strips the raw ID
and substitutes `CILogonID`. That is a v2 regression, not a v1 behavior to
preserve, so copying v1 correctly closes it automatically.

### Specific shape/field traps worth pinning now

Concrete cases behind consequence 3 above, each verified against v1's code:

- v1's `/miscproject/xml` root is a **bare `<Projects>`** with no `xmlns:xsi` and no
  `xsi:schemaLocation`; v2 added both. Any consumer diffing the root element sees
  new attributes.
- v1 `Site.get_tree()` emits `ID, Name, IsCCStar` and then **every remaining
  `SITE.yaml` key sorted alphabetically** — the site element is an open-ended bag,
  not a fixed field list. v2's fixed 13-column projection cannot reproduce a site
  carrying an unmodeled key.
- v1 `_expand_contactlists` calls `move_to_end("ContactRank")` **only in the
  authorized branch**, so contact element ordering legitimately differs between
  authorized and unauthorized responses. Both orders have to be reproduced.
- v1 `_expand_wlcginformation` **deletes** `HEPScore23Percentage` when unset while
  nulling every other field, and defaults `TapeCapacity` to `0` (not `""`).
- v1 drops a resource entirely when it has zero services, and drops a resource
  group entirely when it has zero resources; v2 keeps both and emits empty
  `<Resources></Resources>`, which is actually invalid against `rgsummary.xsd`.
- v1 drops a downtime whose listed services match none of its resource's services,
  and drops the whole downtime if `ResourceName` doesn't resolve. v2 never drops.

### Correction: XML booleans are NOT a divergence

I previously listed Python emitting `True`/`False` vs Go's `true`/`false` as a
parity risk. **That is wrong.** `xmltodict.unparse` renders Python booleans
lowercase, verified against generated v1 output:

```xml
<Active>true</Active>
<Disable>false</Disable>
<IsCCStar>false</IsCCStar>
```

Go's `encoding/xml` produces the same. Booleans match. What *does* differ is the
XML declaration (`utf-8` vs `UTF-8`), indentation (tabs vs 2 spaces), and float
formatting — see `docs/RESOURCE_PARITY.md`.
