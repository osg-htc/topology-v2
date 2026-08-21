# topology-v2 vs. FEATURE_SET.md — Conformance Audit

Audited: 2026-08-10. Spec: `docs/FEATURE_SET.md` (OSG Topology black-box behavioral spec).
Method: 10 parallel research passes over the Go/Next.js codebase, one per spec section, each citing file:line evidence.

**Bottom line: v2 fails the "swap the internals, no consumer would be any the wiser" test.** Several entire subsystems the spec requires do not exist in any form, authentication mechanisms were replaced wholesale, and the Web UI is a different application (authenticated internal console vs. public read-only site). The core data model (Facility/Site/Resource/Downtime/VO) is present but has multiple concrete bugs and unenforced rules. Per the requester's note that write/permissions areas have some tolerance: the change-review area is flagged anyway because it is architecturally, not just cosmetically, different.

---

## 1. Critical gaps — external consumers would break

These are areas the spec explicitly requires to match exactly (external interfaces), and where v2 has no equivalent at all.

### 1.1 StashCache/OSDF credential-generation subsystem — entirely missing
No `/cache/*`, `/origin/*`, `/stashcache/*`, or `/osdf/*` routes exist anywhere in `internal/router/router.go`. No Authfile/SciTokens-config/grid-mapfile generation code exists. No `Namespace`/`DataFederation`/`CredentialGeneration` Go types exist — the DataFederations/StashCache block embedded in VO YAML is never parsed (VOs are stored as opaque raw-YAML blobs, `internal/topology/vos.go`). No CLI equivalent (`cmd/server/main.go` has no subcommands). Requests to these paths fall through to the SPA catch-all and return `200` with the frontend app shell instead of `404`/`503`.
Impact: every external XRootD/Pelican cache and origin server that depends on these endpoints for auth-file and token-issuer configuration would get nothing. This is the single largest functional gap.

### 1.2 Authentication mechanisms replaced wholesale
Spec requires: (1) `Authorization: Bearer tk-<uuid>` API-key auth with hard-fail-no-fallback semantics, and (2) client-certificate DN auth (soft-fail-through) — the two ORed together, equivalent grants. **Neither exists.** Zero occurrences of `Bearer`, `tk-`, `APIKeyHash`, or DN-header handling anywhere in the Go source. v2 instead gates PII on a session-cookie OIDC role check (`OptionalAuth` middleware, `internal/models/auth.go`). Any external caller presenting a bearer token or client cert per the old contract gets no recognition at all — not even a correctly-shaped 401.

### 1.3 Missing routes (confirmed absent from `internal/router/router.go`, 196 lines fully read)
`/rgdowntime/ical`, `/miscuser/xml`, `/api/resource_group_summary`, `/nsfscience/csv`, `/institution_ids`, `/api/institutions` (a differently-shaped `/api/v1/institutions` exists instead — JSON-only, no CSV/Accept-negotiation, wrong row shape), `/collaborations/osg-scitokens-mapfile.conf`, `/oasis-managers/json`, `/metrics` (no Prometheus exporter anywhere in the repo). Also missing as backend routes: `/`, `/map/iframe`, `/resources`, `/collaborations`, `/organizations`, `/contacts`, `/resource-files`, and the three CSRF-protected submission-form routes (`/generate_downtime`, `/generate_resource_group_downtime`, `/generate_project_yaml`) — no CSRF middleware exists in the repo at all.

### 1.4 Cache-Control contract entirely absent
Zero hits for `Cache-Control`/`max-age`/`stale-while-revalidate` across `internal/`. No route sets the spec's `max-age=300, stale-while-revalidate=100`, and none of the contact-bearing endpoints set `private`.

### 1.5 Web UI is a different application
The frontend (`frontend/src/components/AppShell.tsx`) gates every route behind session login and redirects to `/login`. The spec describes a **public, read-only** site (static homepage sitemap, non-interactive Organizations table, searchable Resources/Collaborations browser with a documented micro-syntax, a public Contacts directory, a full-screen embeddable Map) plus lightweight guided-authoring tools layered on top (downtime wizard, project registration — both explicitly *not* auto-submitting in the downtime case). In v2:
- No homepage sitemap — `/` is an authenticated dashboard.
- No Organizations page, no public Resources/Collaborations browser wired to `/miscresource/json`/`/vosummary/json`, no search micro-syntax (v2 uses plain substring filtering), no query persistence/autocomplete.
- No Resource Files page (depends on the missing stashcache-files bundle).
- No public Contacts directory with the spec's gated-field allowlist or staleness banner.
- No Map at all — no mapping library, no map component anywhere in `frontend/src`.
- The downtime wizard is a flat single-step form (no Facility→Site→Resource cascade, no UTC-offset auto-preselect, no 48h-lead advisory) and **submits directly into the real proposal workflow** — the opposite of spec's "guided-authoring only, nothing auto-submitted" contrast with project registration.
- Project registration has no PI First/Last split, no institution/FoS autocomplete, no name auto-generation/locking, no length/character validation, and no external-IdP "sign in to submit automatically" path at all.

### 1.6 Automated change-review / auto-approval policy engine — does not exist
Spec requires an automated policy check that auto-merges a narrow class of pure-downtime, contact-owned, validation-passing changes and otherwise routes to manual review with a specific outcome/feedback taxonomy. v2's `ApproveProposal` (`internal/handlers/proposals.go:203-241`) is 100% human-gated — there is no code path that ever applies a change without a manager/admin clicking approve. No submission-time business-rule validation (Downtime-ID uniqueness, per-record rules, Project reference checks) runs at all; the little validation that exists runs at *apply* time, after a human has already approved. None of the 8 distinct outcome/feedback states in the spec exist — outcomes collapse to generic HTTP success/error. This is flagged despite the stated leniency for write/permission areas because it's a different architecture, not a close variant.

---

## 2. Data-model bugs and unenforced rules (present but diverges)

| Area | Finding | Evidence |
|---|---|---|
| **Resource.Active default** | **Inverted.** Spec: omitted → `true`. v2: every consumer treats `nil` as `false` (`internal/xmlapi/rgsummary.go:303`, `internal/handlers/topology_api.go:147`, `internal/handlers/detail.go:44`). A real functional regression, not just a documentation gap. |
| **Resource.Disable** | Not an independent field — fabricated as `!Active` at output time (`rgsummary.go:329`). No DB column exists. |
| **Resource.Description default** | Spec requires literal `"(No resource description)"` when absent. v2 just omits the field (`omitempty`). String never appears in the repo. |
| **Missing-value placeholder convention** (4 magic strings) | Not implemented anywhere — zero hits repo-wide. |
| **Boolean-string convention** (`1/true/yes/on`, case-insensitive) | Not implemented — filters and Production only accept exact `"1"`/`"on"`/native bool, no `yes`, no case-insensitivity. |
| **ContactLists** | Rank is derived purely from list order, not authoritative; no ID regex (`^[0-9a-f]{40}$`/`^OSG\d+$`) validation; no contact-type enum validation; no case-insensitive name/contact match check; enrichment gates the alt-identity ID behind authorization when spec says it should always be added once resolved. |
| **Facility InstitutionID** | Required unconditionally — the spec's "exempt if zero registered Resources" escape hatch is absent. |
| **Site** | Name regex `^[A-Za-z0-9_ -]+$` not enforced. LongName/AddressLine1/City/Country/Lat/Long are nullable in DB / not in required-schema, despite being "required" per spec. Proposal-path JSON schema uses `additionalProperties:false`, which *rejects* unrecognized fields — spec requires them to be preserved and re-emitted. |
| **Resource Group** | `production`/`support_center` not actually required by schema. Unknown `SupportCenter` name does **not** reject the whole RG (the validation function exists but is never called from the proposal-apply path) — spec says an unknown SupportCenter must reject the entire RG. Malformed individual Resource aborts the *whole* bulk import rather than being skipped with a warning. |
| **Resource: WLCGInformation** | `TapeCapacity` defaults to `""` instead of spec's `"0"`. Everything else (InteropBDII default false, HEPScore23Percentage omit-if-unset) matches. |
| **Resource: Services.Details sub-fields** | `Details` is an untyped `interface{}` blob — none of `hidden`/`uri_override`/`sam_uri`/`endpoint`/etc. exist as real fields; `service_hidden` filtering is consequently entirely absent. |
| **Resource: VOOwnership** | No sum≤100 check, no synthetic `"(Other)"` entry synthesis. |
| **Resource/Downtime/SupportCenter/etc. ID uniqueness** | The identifier-*derivation* formula is correct everywhere (see §3), but literal-ID *global uniqueness* is not enforced at the DB level for Resource or ResourceGroup GroupID. |
| **FQDN uniqueness** | No uniqueness constraint at all (global or XRootD-scoped) — spec wants it scoped to XRootD cache/origin resources specifically. |
| **Downtime.ID** | No global uniqueness constraint in DB. Read-time ID-derivation-from-CreatedTime formula (`int((created_epoch-1_535_000_000)*10)`) doesn't exist anywhere; the submission-form path mints a plain `time.Now().Unix()` instead — doesn't match spec's formula and is inconsistent with the (non-existent) read-time fallback. |
| **Downtime.Class/Severity/Description** | Accepted as free text with no requiredness enforcement (Severity-not-a-closed-set correctly matches spec; but Description/Class aren't actually required either, which they should be). |
| **Downtime date formats** | Two different, both-incomplete implementations across `rgdowntime.go` and `proposals.go`; neither covers all 4 spec formats, and the submission path additionally accepts RFC3339/datetime-local shapes not in spec at all. |
| **Downtime canonical output format** | v2's stored/rendered format (`Jan 02, 2006 15:04 -0700`) does not match spec's `"%b %d, %Y %H:%M %p %Z"` (no AM/PM, no zone name) — and nothing validates submissions against the canonical format at all. |
| **Downtime malformed-entry handling** | A bad entry aborts the entire tree read (`return err`) rather than being skipped individually. |
| **Downtime cancel semantics** | Cancel is a soft-delete (`deleted_at` set) — spec requires full removal, not retained history. |
| **Downtime.end_age** | Not implemented at all. |
| **Downtime.ResourceName/Services cross-checks** | Unresolvable ResourceName silently produces a zero-value entry instead of being skipped; Services-belong-to-resource is never checked. |
| **VO.Contacts / TokenIssuers.Pattern** | Contact struct has Email/Phone/SMSAddress/DN fields but they're never populated regardless of auth (permanently empty, not gated). No `Pattern` field computed for JSON TokenIssuers (spec's XML/JSON-divergence claim doesn't materially apply since JSON is actually a raw-YAML dump with a different shape entirely). |
| **VO.ReportingGroups** | Always renders empty — the field is never read from source data at all (typed as an empty struct). A real `REPORTING_GROUPS.yaml` source file exists but nothing in the Go code reads it. |
| **VO.OASIS.Managers** | Only the list shape is handled; the legacy map shape (`Name → {ID,DNs}`) is silently dropped — confirmed against real data using the map shape. |
| **VO.ParentVO** | Extraction is broken against real data: code expects a string but real YAML stores `{ID,Name}` as a map, so `ParentVO.Name`/`ID` are always empty in practice. |
| **VO.DataFederations.StashCache** | Correctly absent from `/vosummary/xml` output (this one is a genuine match — but only because the whole feature is unimplemented, see §1.1). |
| **Project business rules** | Sponsor exclusivity, CampusGrid-registry resolution, VO-exact-match, ResourceAllocations references (SubmitResources/ExecuteResourceGroups), PIName format — none enforced. Organization/Description/FieldOfScience/PIName are all optional despite being "required" per spec. |
| **Project.ID** | Unlike every other entity type, Project does **not** get the shared `GenID` md5-formula fallback when no literal ID is supplied — renders with an empty ID instead. |
| **Mappings (4 reference tables)** | NSF-category mapping, field-of-science taxonomy, and legacy institution-name lookup: none exist (no tables, no files, no routes). Institution registry exists and is live-synced from an external API (a genuine match for that one table) — but `name` has no uniqueness constraint (only `id` does), contradicting spec's "both globally unique." |
| **Contact model** | Drastically thinner than spec. Only `Name/CILogonID/Email/ContactRank` exist — no SecondaryEmail, phones, IM, SMSAddress, DNs, ContactPreference, APIKeyHash, Flags, GitHub, Profile, PhotoURL. No Gravatar computation. No federated LDAP/CILogon live merge (Contact = simple DB row, not a two-source merge). No dedup-by-CILogonID rule. |
| **Contact hash purposes** | Contact-ID minting (SHA-1 of lowercased+trimmed email) is correctly implemented and tested. The second, distinct credential-artifact X.509-subject-DN hash does **not** exist (consistent with §1.1's total absence of credential generation). |
| **API-key auth** | Entirely absent — no `sha256:` hash format, no separate key-mapping table, no bearer auth at all (see §1.2). Fail-stale semantics can't be evaluated because the feature doesn't exist. |

---

## 3. What does match well

- **Identifier-derivation formula** (`1 + md5(name) % (2^31-1)`) — exact match, applied consistently for Facility/Site/ResourceGroup/Resource/SupportCenter/Service via `topology.GenID` (`internal/topology/model.go:20-26`). This is the one piece of "load-bearing magic" that was carried over correctly.
- **Contact-ID minting** (SHA-1 of lowercased+trimmed email) — exact match, unit-tested.
- **FQDN required / hard failure if missing** — matches.
- **CC\* tag propagation** Resource→ResourceGroup→Site→Facility — matches.
- **Site-name and ResourceGroup-name global-uniqueness DB constraints** — match (though the *format* regex for Site names is missing, see §2).
- **Resource Group Disable-always-false** at the RG level — matches.
- **`/schema/<name>` endpoint** — exact match: exactly 5 recognized XSD files, 404 on unknown name.
- **Core read routes exist and roughly work**: `/rgsummary/xml`, `/rgdowntime/xml`, `/vosummary/xml`, `/vosummary/json`, `/miscproject/xml`/`json`, `/miscsite/json`, `/miscfacility/json`, `/miscresource/json` are all registered and return plausible data (though see per-endpoint deviations in §2 and the missing 400/plaintext error contract in §1).
- **rgsummary/xml field-level PII gating** for resource contacts (Email gated by role, name public) — the *mechanism* is structurally right, even though the field set is incomplete (no Phone/SMSAddress/DN fields exist to gate).
- **Downtime timeframe bucketing (Past/Current/Future) and hardcoded `UpdateTime: "Not Available"`** — match.
- **gridtype "both = neither" quirk** — correctly reproduced (though gated behind an extra key not in spec).
- **WLCGInformation's `HEPScore23Percentage`-omitted-if-unset special case** — correctly distinguished from the other WLCG defaults.
- **API-key self-service absence** — trivially matches spec's "no self-service issuance/rotation/revocation endpoint," though only because no API-key mechanism exists at all.

---

## 4. Severity summary

**Would break external consumers today (must-fix if this is meant to be a drop-in replacement):**
StashCache/OSDF subsystem (§1.1), bearer/cert authentication (§1.2), missing routes incl. `/metrics` and the ical/miscuser/oasis-managers/institutions/nsfscience feeds (§1.3), Cache-Control contract (§1.4), Resource.Active inverted default (§2).

**Architecturally different, not just incomplete (flagged despite stated leniency on write/permission areas):**
Web UI is a different application entirely (§1.5), automated change-review/auto-approval doesn't exist (§1.6).

**Real but contained data-model gaps** (§2): Downtime ID/format/lifecycle issues, Resource Group SupportCenter-rejection not wired up, VO ReportingGroups/OASIS-legacy-shape/ParentVO bugs, Project business-rule validation absent, Contact model far thinner than spec, Mappings tables mostly absent.

**Solid matches** (§3): identifier derivation, contact-ID minting, FQDN/CC\*/uniqueness basics, schema endpoint, core route skeleton, PII field-gating mechanism.

---

## 5. Verification against topology-v1 (the actual original implementation)

Everything above was checked against `docs/FEATURE_SET.md`, a spec written to describe v1's behavior. To make sure the spec itself wasn't idealizing or misdescribing v1, a second pass re-verified every finding above directly against v1's real source at `/Users/mriggle/Coding/topology/` (Python/Flask app in `src/` and `src/webapp/`, GitHub Actions in `.github/workflows/`, and git history of the hand-edited YAML dataset).

**Result: the overwhelming majority of findings are confirmed genuine — v1 really does implement the behavior the spec describes, and v2 really does lack or diverge from it.** In particular, three of the largest gaps identified above are now confirmed with high confidence, file-and-line, against v1's actual code:

- **StashCache/OSDF credential generation** exists in v1 in full, production-grade detail: every route (`/cache/*`, `/origin/*`, `/stashcache/*`, `/osdf/*`), the literal `u * /user/ligo -rl` deny line, the dummy `scitokens.org/nonexistent` fallback issuer, the exact 8000/8443/1094/1095 default ports, the DN-hash regex, and standalone CLI equivalents (`bin/osg-authfile` etc.) all verified present and working in `src/stashcache.py`. v2 has none of it. This is a confirmed, complete regression.
- **Automated change-review** exists in v1 as a rich, precise mechanism: a GitHub webhook app (`src/webhook_app.py`) triggers a policy check (`src/webapp/automerge_check.py`) with exactly the preconditions the spec describes (downtime-only, per-resource contact ownership, hardcoded-to-production identity lookup, hard Project-org/institution reject with no bypass) and a six-state outcome taxonomy verified 1:1 against `webhook_status_messages.py`. v2's fully-manual approval flow is a confirmed, genuine architectural departure — not a spec-text artifact.
- **The public Web UI** (homepage sitemap, lunr.js-powered search with the exact micro-syntax, localStorage query history, 576px responsive breakpoint, contacts-gating, the full Leaflet map with view-selector/legend/jitter/embed-mode) all exist and were verified line-by-line in v1's Jinja2 templates and `static/js/`. v1's HTML routes are genuinely unauthenticated. v2's authenticated-console frontend is a confirmed, real divergence.

### Corrections — cases where the original finding was overstated or needs a caveat

A smaller number of points turned out to be inaccurate once checked against v1 directly — either the spec text overstated v1's actual rigor, or v1 itself doesn't enforce something the spec implies it does. These should be treated as **not genuine v1-vs-v2 differences** (both versions share the gap) or **downgraded/re-scoped**:

| # | Area | Correction |
|---|---|---|
| 1 | ContactType/ContactRank vocab | v1 does **not** enforce the ContactType or ContactRank vocab either — it's convention/documentation only (`template-resourcegroup.yaml`), checked nowhere in code. Real production data uses different values. **Not a genuine v2 regression.** |
| 2 | Site required fields (LongName/AddressLine1/City/Country/Lat/Long) | v1 does not enforce any of these as required — only that a `SITE.yaml` file exists. **Not a genuine v2 regression.** |
| 3 | Resource Group `Production` required | v1 does not require it either — missing `Production` just defaults to falsy (ITB), no rejection. **Not a genuine v2 regression** (the "required" framing was overstated for v1 too). |
| 4 | Support Center contacts validated like Resource ContactLists | v1 has this check implemented but **explicitly disabled** in code (commented out, citing ticket SOFTWARE-3329). **Not currently a genuine v2 regression** — v1 doesn't run this check either right now. |
| 5 | Resource `Tags: ["OSPool"]` propagation | v1's real OSPool designation comes from an external maintained list matched by resource/site/RG **name** inside the map's JS (`iframe.html.j2`), not from the Tags field at all. The spec's Tags-based framing doesn't match v1's actual mechanism. v2 is still missing OSPool support overall (consistent with its missing Map), but not via this specific claimed mechanism. |
| 6 | `Services.<Name>.Details.{endpoint_override, auth_endpoint_override}` | These two sub-fields do not exist anywhere in v1 (template, code, or data) — likely introduced in error during spec-writing. `uri_override`/`sam_uri`/`endpoint` exist only as commented-out YAML examples, never read by any v1 code. Only `hidden` is a real, enforced field in v1. **Significantly overstated** — v1 itself treats `Details` as an almost-entirely-opaque blob. |
| 7 | Downtime ID derived from `CreatedTime` at **read time** if missing | v1 has no such read-time fallback — a missing ID simply causes that entry to be skipped. The `1_535_000_000`-based formula is real but is only used when the **submission form** mints a brand-new ID from the current time. **Partially overstated**; the formula and its submission-time use are genuine, the read-time-fallback framing is not. |
| 8 | Downtime `Services` must belong to the named Resource | v1 only checks service names against the *global* services catalog at CI-validation time; the resource-specific match happens silently at **render** time (a downtime whose services don't match its resource's services is just dropped from output, not flagged as invalid). **Re-scoped**, not a straightforward validation rule. |
| 9 | Downtime canonical output format checked before a submission is accepted | v1 does not validate submissions against the canonical output format string — submissions use a separate format constant entirely. The output format itself is genuine; "checked before acceptance" is **not genuine**. |
| 10 | `gridtype` filter requiring an extra gating key is a v2-only deviation | Corrected: **v1 also gates `gridtype_1`/`gridtype_2` behind a `gridtype` key being present** (`app.py:1087`). v2's behavior here actually matches v1 — this should be removed from the deviation list entirely. |
| 11 | Cert-DN auth mechanism | v1 doesn't use `SSL_CLIENT_S_DN`; it reads GridSite-convention `GRST_CRED_AURI_*` WSGI-environ variables. The soft-fail-through *behavior* is confirmed genuine; the specific header/variable name in the original finding was a guess and is corrected here. |
| 12 | PI Name format (`"<First> <Last>"`) validated | v1 never validates this format on submitted/hand-edited Project YAML — it's only assembled that way when the web form *generates* a new record. **Not a genuine v2 regression** as a validation rule. |
| 13 | Project Sponsor CampusGrid/VO mutual exclusivity enforced | v1 does not actually enforce exclusivity — if both `CampusGrid` and `VirtualOrganization` are present, `CampusGrid` silently wins with no error. **Overstated** for v1 as well. |
| 14 | `ResourceAllocations[].Type` restricted to "XRAC"/"Other" | Not enforced as an enum in v1 either. **Not a genuine v2 regression.** |
| 15 | `FieldOfScienceID` validated against the FoS taxonomy | In v1 this is only a UI-dropdown convenience at web-form submission time, not a data-integrity check anywhere (`verify_projects.py` doesn't check it). **Overstated** as a "validation rule." |
| 16 | Field-of-science taxonomy has "~500 entries" | Actual count in v1's `mappings/field_of_science.yaml` is **1,560 entries**. Minor factual correction to the spec text, not a v2-specific finding. |
| 17 | Origin Authfile: authenticated lines "precede" public lines in one file | v1 actually generates `/origin/Authfile` and `/origin/Authfile-public` as two **separate, mutually exclusive** outputs (a boolean flag picks one branch or the other) — there's no single combined file where ordering "precedes" anything. Re-scoped; not a meaningful ordering rule in current v1 behavior. |
| 18 | Grid-mapfile DN-hash 3-individual carve-out is a hardcoded production exception | It's actually a **test-fixture-only** allowance (`test_stashcache.py`) documenting known real-world exceptions in the dataset, not logic hardcoded into the generator itself. |
| 19 | "Environment badge" on non-production instances | v1 has no visual badge component — non-production is signaled by plain instance-name text appended to the page `<title>`/`<h1>` via config. Real but much less than "badge" implies. |
| 20 | Downtime-wizard advisory lead-time is "≥48h" | v1's actual threshold is **24 hours**, not 48 (`forms.py`, label reads "registered at least 24 hours in advance"). Corrects the spec text; v2 lacks the advisory note regardless of threshold. |
| 21 | Project-registration OAuth return-URL is checked against "a known allowlist" | v1 does not implement a coded allowlist — the return URL is simply round-tripped from the same-origin `Referer` header with no explicit validation. The open-redirect risk framing doesn't apply cleanly to v1 either, since v2 has no such feature at all to compare against. |

None of these corrections weaken the report's central conclusions — StashCache/OSDF, automated change-review, and the public Web UI remain confirmed, severe, genuine gaps in v2. The corrections above mostly soften a handful of secondary data-model validation claims where v1 turned out to be looser than the spec implied.

---

## 6. Live verification — both servers run side by side

Both versions were actually brought up and tested against each other, not just read as code. v1 was started with its Flask dev server (`FLASK_ENV=development python src/app.py`, port 9000) directly against the real topology git checkout. v2 was brought up via `docker compose up --build` and loaded with the exact same source data via `topology-server import-tree` (confirmed matching: 1099 resources on both). Every finding below is a live HTTP/browser observation, not a code reading.

**This live pass did two things: it reproduced nearly every static finding with concrete evidence, and it surfaced several bugs that only show up at runtime.**

### New, more severe findings only visible live

- **`/map/iframe` and `/stashcache/namespaces` don't just fall back gracefully on v2 — they infinite-loop.** Loading `/map/iframe` in an actual browser against v2 produces `net::ERR_TOO_MANY_REDIRECTS` and a blank black screen, not a degraded-but-working page. Same for `/stashcache/namespaces` (`curl -L` hits curl's redirect-limit error). This is worse than "route missing" — it's a client-visible hang.
- **v2's `/rgdowntime/xml` default response is ~250x larger than v1's and dumps the entire historical downtime dataset.** Because `downtime_attrs_showpast` is a total no-op in v2, a default (no-filter) request returns v1's default of 15 current entries vs. **v2's 3,776 entries, 3,760 of them past** — the exact same as its own `?downtime_attrs_showpast=all`. Any consumer expecting v1's "just show me what's live" default gets a multi-megabyte historical dump instead.
- **The `Active` default bug has a measured, real-data blast radius: 18 real resources.** Comparing all 1099 resources by ID, 18 flip from `Active=true` in v1 to `Active=false` in v2 (e.g. `CIT_LIGO_ORIGIN`, ID 948) — a concrete count, not a theoretical concern.
- **The fabricated `Disable = !Active` affects 338 resources.** Every resource in the real dataset with `Active=false, Disable=false` in v1 (independent fields) renders as `Disable=true` in v2.
- **v2 never returns a real 404.** Every unrecognized path — including a deliberately nonsense one (`/this-path-does-not-exist-at-all`) — gets `301 → Location: ./`, redirecting to the SPA shell. v1 returns a genuine Flask 404 for the same request.
- **v2 has essentially no server-side input validation on filters, live-confirmed across seven distinct cases**: `active_value=7` (invalid enum) → v1 400s, v2 silently 200s; `disable=on&disable_value=1` → v1 filters, v2 no-ops (returns everything); `has_wlcg=1` → v1 narrows 429→84 resource groups, v2 stays at 429 (no-op); `downtime_attrs_showpast=notanumber` → v1 400s with an explicit message, v2 200s with the full dataset; malformed/unknown bearer tokens → v1 401s immediately, v2 ignores the header and serves the request normally.
- **`/api/v1/institutions` on v2 is a stub that returns `[]`.** It's a real, routed endpoint (unlike the others, which all 301), but it returns no data at all — worth flagging separately from "endpoint missing," since it could look superficially fine in a smoke test.
- **CORS is global and Origin-gated on v2, strictly per-route on v1.** With no `Origin` header, neither server sends `Access-Control-Allow-Origin`. But send any `Origin` header and v2 adds `Access-Control-Allow-Origin: *` to **every** route, including `/rgsummary/xml` — a route v1 never CORS-enables regardless of headers sent.
- **v1 itself has a real display bug worth knowing about**: its "canonical" downtime time format retains 24-hour hours but still appends a nonsensical `PM` suffix (e.g. `13:38 PM`, not the clean `01:38 PM` the spec's own example implied). v2's actual format (`13:38 +0000`) is at least internally consistent, even though it doesn't match v1's intended format.

### Static findings that fully reproduced live (high-confidence, with real data)

`Description` default (`(No resource description)`: 24 occurrences in v1, 0 in v2) · `WLCGInformation.TapeCapacity` (v1 emits `0` for unset, v2 emits empty) · `VOOwnership` synthetic `(Other)` entry (44 in v1, 0 in v2) · OASIS legacy-map `Managers` (real VO `ARA` — populated in v1, empty in v2) · `ParentVO` map shape (real VOs `Nova`/`ARA` — populated in v1, empty in v2) · `ReportingGroups` expansion (real VO `LIGO` — full contact/FQAN data in v1, empty in v2) · `TokenIssuers.Pattern` JSON-only field (present in v1's `/vosummary/json`, absent in v2's) · v2's JSON not being a re-encoding of its own XML (confirmed: different key casing/nesting between v2's own two formats) · no `Cache-Control` header anywhere on v2 · no `private` marking on v2's (non-existent) contact endpoints · plaintext `400 "Invalid arguments: ..."` on v1 vs. silent `200` on v2 · missing StashCache/OSDF routes and their specific literal content (the `u * /user/ligo -rl` line, the `scitokens.org/nonexistent` dummy fallback, 1094/1095 ports, DN-hash grid-mapfile lines, the scitokens mapfile, CSV/JSON institutions negotiation) — all confirmed present and real in v1's live output, all confirmed absent in v2's.

### Static findings that did NOT reproduce live (real matches, despite the earlier claim)

- **FQDNAlias rendering** — byte-identical between v1 and v2 for every resource checked (71 alias tags, 53 non-empty blocks, both sides).
- **Unauthenticated ContactLists enrichment** — neither server exposes CILogonID/email/phone without auth; both correctly withhold it. (The static audit's concern about *authorized* enrichment gating couldn't be tested live without real auth credentials — this remains a code-level finding only.)
- **CC\* cascade** — identical `IsCCStar` propagation through Resource→ResourceGroup→Site→Facility on both servers for a real CC\*-tagged resource.
- **`gridtype` "both = neither" rule and its outer gating key** — both servers require `gridtype=on` before consulting `gridtype_1`/`gridtype_2`, and both correctly no-op when both sub-flags are set. (Confirms the earlier correction that this was never a real v1-vs-v2 deviation.)
- **`DataFederations` exclusion from `/vosummary/xml`** — zero occurrences on both.
- **VO contact Email/Phone gating (unauthenticated)** — zero on both.
- **Downtime `ID` and `UpdateTime`** — identical IDs for matching entries; both always render `UpdateTime` as the literal `"Not Available"`.
- **Bogus/unrecognized query parameters** — both servers silently ignore them (200, unfiltered).
- **XML/JSON `Content-Type` headers** — match on both for `/rgsummary/xml` and `/miscresource/json`.

**Net effect of the live pass:** it does not change the report's conclusions — if anything, it sharpens them. The three headline gaps (StashCache/OSDF, automated change-review, public Web UI) are now confirmed with actual request/response evidence, not just code reading, and several of the data-model bugs turned out to have larger, more concrete real-world impact than the static read suggested (the `Active`/`Disable` bugs affect hundreds of real resources; the downtime-history bug turns a 15-entry default response into a 3,776-entry one). A handful of narrower claims (FQDNAlias, CC\* cascade, `gridtype` gating) turned out to be non-issues once tested against real traffic.
