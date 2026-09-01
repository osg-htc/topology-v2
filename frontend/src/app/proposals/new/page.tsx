"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card, btn, btnSecondary, input, label } from "@/components/ui";
import { MultiSelect } from "@/components/MultiSelect";
import { ContactPicker } from "@/components/ContactPicker";
import { ParentChainPicker, Placement, BundleOp } from "@/components/ParentChainPicker";

const CONTACT_TYPES = [
  "Administrative Contact",
  "Security Contact",
  "Executive Contact",
  "Local Operational Contact",
  "Local Security Contact",
];
const RANKS = ["Primary", "Secondary", "Tertiary"];
const COMMON_TAGS = ["OSPool", "CC*"];

// A hostname must be a valid (dotted) DNS name.
const HOSTNAME_RE =
  /^(?=.{1,253}$)([a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,63}$/;
const isHostname = (h: string) => HOSTNAME_RE.test(h.trim());

// Rank is derived from a contact's order within its type (1st = Primary, …); the
// UI only exposes ordering. External ids are never shown — a contact is either
// picked from existing users or onboarded via an invite (invitePending).
// Capped at 3 rows per type in this form (see MAX_PER_TYPE below): the wire
// shape (map[type][rank]Contact) can only hold one entry per rank, so a 4th
// same-type row would silently overwrite the 3rd rather than erroring.
type ContactRow = { type: string; name: string; id: string; inviteId?: string; invitePending?: boolean; inviteUrl?: string };
const MAX_PER_TYPE = RANKS.length;
type ServiceRow = { name: string; description: string };

function NewResourceForm() {
  const router = useRouter();
  const params = useSearchParams();
  const editParam = params.get("edit");
  // A resource's edit target is its immutable topology_id, not its (mutable)
  // name -- so a rename can never break the link or duplicate the row.
  const editId = editParam != null ? Number(editParam) : null;
  const [name, setName] = useState("");
  // In edit mode the resource group is fixed; in create mode ParentChainPicker
  // resolves the placement and any inline parent-creation operations.
  const [placement, setPlacement] = useState<Placement>({ rg: params.get("rg") ?? "", ops: [], valid: false });
  const rg = placement.rg;
  const [hostname, setHostname] = useState("");
  const [description, setDescription] = useState("");
  const [active, setActive] = useState(false);
  const [aliases, setAliases] = useState<string[]>([]);
  const [tags, setTags] = useState<string[]>([]);
  const [allowedVOs, setAllowedVOs] = useState<string[]>([]);
  const [contacts, setContacts] = useState<ContactRow[]>([
    { type: "Administrative Contact", name: "", id: "" },
    { type: "Security Contact", name: "", id: "" },
  ]);
  const [services, setServices] = useState<ServiceRow[]>([{ name: "", description: "" }]);
  const [advanced, setAdvanced] = useState(false);
  const [advancedJSON, setAdvancedJSON] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [showErrors, setShowErrors] = useState(false);

  // Edit mode: load the existing resource and prefill.
  const { data: editData } = useQuery({
    queryKey: ["resource-detail", editId],
    queryFn: () => api.resourceDetail(editId!),
    enabled: editId != null,
  });
  useEffect(() => {
    if (!editData) return;
    setName(editData.name);
    setPlacement({ rg: editData.resource_group, ops: [], valid: true });
    setHostname(editData.fqdn);
    setDescription(editData.description);
    setActive(editData.active);
    setAliases(editData.fqdn_aliases ?? []);
    setTags(editData.tags ?? []);
    setAllowedVOs(editData.allowed_vos ?? []);
    if (editData.services?.length)
      setServices(editData.services.map((s) => ({ name: s.name, description: s.description })));
    if (editData.contacts?.length) {
      // Order contacts within each type by their existing rank.
      const rankIdx = (r: string) => Math.max(0, RANKS.indexOf(r));
      const ordered = [...editData.contacts].sort(
        (a, b) => a.contact_type.localeCompare(b.contact_type) || rankIdx(a.rank) - rankIdx(b.rank),
      );
      setContacts(ordered.map((c) => ({ type: c.contact_type, name: c.name, id: c.id })));
    }
  }, [editData]);

  const { data: serviceNames } = useQuery({ queryKey: ["service-names"], queryFn: api.serviceNames });
  const { data: voNames } = useQuery({ queryKey: ["vo-names"], queryFn: api.voNames });
  const { data: tagNames } = useQuery({ queryKey: ["tag-names"], queryFn: api.tagNames });

  const rgValid = placement.valid;
  // A row counts once it's linked to a real person (a real id) or has a
  // pending invite; a typed-but-never-picked name is neither, so it can't
  // satisfy either check below -- matching what the backend enforces at
  // apply time.
  const contactsResolved = contacts.every((c) => (!c.name && !c.id) || !!c.id || !!c.invitePending);
  const hasContact = contacts.some((c) => c.id || c.invitePending);
  const tagOptions = Array.from(new Set([...COMMON_TAGS, ...(tagNames ?? [])]));
  const countsByType = contacts.reduce<Record<string, number>>((acc, c) => {
    acc[c.type] = (acc[c.type] ?? 0) + 1;
    return acc;
  }, {});

  // Every field this form actually renders is always included below, even
  // when empty -- the backend now merges a submission onto the resource's
  // current state, keyed by field presence, so omitting a key means "leave
  // whatever this already had," not "empty." A field the form has no input
  // for at all (e.g. DN, VOOwnership) is correctly never mentioned here,
  // which is exactly what keeps it safe.
  const buildResource = (): Record<string, unknown> => {
    const resource: Record<string, unknown> = { FQDN: hostname, Active: active };
    resource.Description = description;
    resource.FQDNAliases = aliases;
    resource.Tags = tags;
    resource.AllowedVOs = allowedVOs;

    const svc: Record<string, unknown> = {};
    for (const s of services) if (s.name) svc[s.name] = s.description ? { Description: s.description } : {};
    resource.Services = svc;

    // Rank is derived from order within each contact type (1st = Primary, …).
    const cl: Record<string, Record<string, unknown>> = {};
    const perType: Record<string, number> = {};
    for (const c of contacts) {
      if (!c.name && !c.id) continue;
      const n = perType[c.type] ?? 0;
      perType[c.type] = n + 1;
      const rank = RANKS[Math.min(n, RANKS.length - 1)];
      cl[c.type] = cl[c.type] || {};
      cl[c.type][rank] = { Name: c.name, ID: c.id };
    }
    resource.ContactLists = cl;
    return resource;
  };

  // Validation state for red-marking on submit.
  const badHostname = hostname !== "" && !isHostname(hostname);
  const badAliases = aliases.some((a) => !isHostname(a));
  const invalid = {
    name: !name,
    rg: !rgValid,
    hostname: !hostname || badHostname,
    contact: !hasContact || !contactsResolved,
  };
  const errCls = (bad: boolean) => (showErrors && bad ? " border-red-500 ring-1 ring-red-400" : "");

  const toggleAdvanced = () => {
    if (!advanced) setAdvancedJSON(JSON.stringify(buildResource(), null, 2));
    setAdvanced(!advanced);
  };

  const submit = async (asDraft: boolean) => {
    setErr("");
    // Full validation only when submitting for review (drafts may be incomplete).
    if (!asDraft && !advanced) {
      if (invalid.name || invalid.rg || invalid.hostname || invalid.contact || badAliases) {
        setShowErrors(true);
        setErr("Please fix the highlighted fields.");
        return;
      }
    }
    setBusy(true);
    try {
      let resource: unknown;
      if (advanced) {
        resource = JSON.parse(advancedJSON);
      } else {
        resource = buildResource();
      }
      const resourceOp: BundleOp = {
        entity_kind: "resource",
        operation: editId != null ? "update" : "create",
        target_name: editId != null ? String(editId) : undefined,
        proposed_state: { name, resource_group: rg, resource },
      };
      // Onboarding invites for any brand-new contacts block approval until accepted.
      const pending = contacts.filter((c) => c.invitePending && c.inviteId).map((c) => c.inviteId!);
      // If the user created any parents inline, submit one atomic bundle with the
      // parents ordered before the resource; otherwise a plain resource proposal.
      const body =
        placement.ops.length > 0
          ? {
              entity_kind: "bundle",
              operation: "create",
              submit: !asDraft,
              proposed_state: { operations: [...placement.ops, resourceOp] },
              pending_invite_ids: pending,
            }
          : { ...resourceOp, submit: !asDraft, pending_invite_ids: pending };
      const res = await api.proposals.create(body);
      router.push(`/proposals/view?id=${res.id}`);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const setContact = (i: number, patch: Partial<ContactRow>) =>
    setContacts(contacts.map((c, j) => (j === i ? { ...c, ...patch } : c)));
  const removeContact = (i: number) => setContacts(contacts.filter((_, j) => j !== i));
  const moveContact = (i: number, dir: -1 | 1) => {
    const j = i + dir;
    if (j < 0 || j >= contacts.length) return;
    const next = [...contacts];
    [next[i], next[j]] = [next[j], next[i]];
    setContacts(next);
  };
  const setService = (i: number, patch: Partial<ServiceRow>) =>
    setServices(services.map((s, j) => (j === i ? { ...s, ...patch } : s)));

  return (
    <div className="p-8">
      <PageHeader
        title={editId != null ? `Edit resource: ${name || editId}` : "Register a resource"}
        description="Provide the resource's placement, host name, services, and contacts. At least one contact is required for a complete registration."
      />
      <div className="max-w-2xl space-y-6">
        <Card>
          <h3 className="mb-3 text-sm font-semibold text-gray-700">Basics</h3>
          <div className="space-y-4">
            <div>
              <label className={label}>Resource name</label>
              <input className={input + errCls(invalid.name)} value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. UChicago_OSGConnect_ap20" />
            </div>
            {editId != null ? (
              <div>
                <label className={label}>Resource group</label>
                <input className={`${input} bg-gray-50 text-gray-500`} value={rg} disabled />
                <p className="mt-1 text-xs text-gray-400">A resource cannot be moved between groups here.</p>
              </div>
            ) : (
              <ParentChainPicker
                value={placement.rg}
                invalid={showErrors && invalid.rg}
                onResolve={setPlacement}
              />
            )}
            <div>
              <label className={label}>Host name</label>
              <input className={input + errCls(invalid.hostname)} value={hostname} onChange={(e) => setHostname(e.target.value)} placeholder="host.example.org" />
              {badHostname && <p className="mt-1 text-xs text-red-600">Must be a valid DNS host name (e.g. host.example.org).</p>}
            </div>
            <div>
              <label className={label}>Host name aliases</label>
              <MultiSelect options={[]} value={aliases} onChange={setAliases} placeholder="Add an alias and press Enter…" />
              {badAliases && <p className="mt-1 text-xs text-red-600">Every alias must be a valid DNS host name.</p>}
            </div>
            <div>
              <label className={label}>Description</label>
              <textarea className={input} rows={2} value={description} onChange={(e) => setDescription(e.target.value)} />
            </div>
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input type="checkbox" checked={active} onChange={(e) => setActive(e.target.checked)} />
              Active (accepting requests)
            </label>
          </div>
        </Card>

        <Card>
          <h3 className="mb-3 text-sm font-semibold text-gray-700">Tags &amp; VOs</h3>
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label className={label}>Tags</label>
              <MultiSelect options={tagOptions} value={tags} onChange={setTags} placeholder="Pick a tag…" />
            </div>
            <div>
              <label className={label}>Allowed VOs</label>
              <MultiSelect options={voNames ?? []} value={allowedVOs} onChange={setAllowedVOs} allowCustom={false} placeholder="Pick a VO…" />
            </div>
          </div>
        </Card>

        <Card>
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-gray-700">Services</h3>
            <button className="text-xs text-brand-600 hover:underline" onClick={() => setServices([...services, { name: "", description: "" }])}>
              + Add service
            </button>
          </div>
          <div className="space-y-2">
            {services.map((s, i) => (
              <div key={i} className="grid grid-cols-2 gap-2">
                <select className={input} value={s.name} onChange={(e) => setService(i, { name: e.target.value })}>
                  <option value="">Select a service…</option>
                  {(serviceNames ?? []).map((n) => (
                    <option key={n} value={n}>{n}</option>
                  ))}
                </select>
                <input className={input} placeholder="Description" value={s.description} onChange={(e) => setService(i, { description: e.target.value })} />
              </div>
            ))}
          </div>
        </Card>

        <Card>
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-gray-700">Contacts</h3>
            <button
              className="text-xs text-brand-600 hover:underline disabled:opacity-30"
              disabled={CONTACT_TYPES.every((t) => (countsByType[t] ?? 0) >= MAX_PER_TYPE)}
              onClick={() => {
                const openType = CONTACT_TYPES.find((t) => (countsByType[t] ?? 0) < MAX_PER_TYPE) ?? CONTACT_TYPES[0];
                setContacts([...contacts, { type: openType, name: "", id: "" }]);
              }}
            >
              + Add contact
            </button>
          </div>
          {showErrors && invalid.contact && (
            <p className="mb-2 text-xs text-red-600">
              {hasContact ? "Every contact must be linked to a person — search and select, or invite a new one." : "At least one contact is required."}
            </p>
          )}
          <div className="space-y-2">
            {contacts.map((c, i) => (
              // Keying on index alone means a row that starts blank (the
              // default rows, before the edit-mode prefill fetch resolves)
              // keeps its already-mounted ContactPersonInput -- whose text
              // state is seeded from props only once, on mount -- so it
              // never picks up the prefilled name once editData arrives.
              // Including id forces a remount exactly when a row's
              // underlying identity actually changes (prefill or a fresh
              // pick), not on every keystroke (id stays "" while typing).
              <div key={`${i}-${c.id}`} className="flex items-start gap-2">
                <div className="flex flex-col pt-2 text-gray-400">
                  <button type="button" className="leading-none hover:text-gray-700 disabled:opacity-30" disabled={i === 0} onClick={() => moveContact(i, -1)} aria-label="Move up">▲</button>
                  <button type="button" className="leading-none hover:text-gray-700 disabled:opacity-30" disabled={i === contacts.length - 1} onClick={() => moveContact(i, 1)} aria-label="Move down">▼</button>
                </div>
                <div className="grid flex-1 grid-cols-2 gap-2">
                  <select className={input} value={c.type} onChange={(e) => setContact(i, { type: e.target.value })}>
                    {CONTACT_TYPES.map((t) => (
                      <option key={t} value={t} disabled={t !== c.type && (countsByType[t] ?? 0) >= MAX_PER_TYPE}>
                        {t}
                      </option>
                    ))}
                  </select>
                  <ContactPicker value={c} onChange={(patch) => setContact(i, patch)} />
                </div>
                <button type="button" className="pt-2 text-gray-300 hover:text-red-600" onClick={() => removeContact(i)} aria-label="Remove contact">×</button>
              </div>
            ))}
          </div>
          <p className="mt-2 text-xs text-gray-400">
            Order within a contact type sets its rank (Primary, Secondary, Tertiary; up to {MAX_PER_TYPE} per type) — use ▲▼ to reorder.
            Type to search people; selecting one links the contact, or invite someone new.
          </p>
        </Card>

        <Card>
          <button className="text-xs text-gray-500 hover:text-gray-800" onClick={toggleAdvanced}>
            {advanced ? "▾ Hide advanced (raw JSON)" : "▸ Advanced edit (raw JSON — for rare fields like DN)"}
          </button>
          {advanced && (
            <div className="mt-3">
              <p className="mb-2 text-xs text-gray-400">
                Edits here override the fields above on submit. This is the only place raw JSON is
                used — for uncommon fields (e.g. host-certificate DN, WLCG info).
              </p>
              <textarea
                className={`${input} font-mono text-xs`}
                rows={14}
                value={advancedJSON}
                onChange={(e) => setAdvancedJSON(e.target.value)}
              />
            </div>
          )}
        </Card>

        {err && <p className="text-sm text-red-600">{err}</p>}
        <div className="flex gap-3">
          <button className={btn} disabled={busy} onClick={() => submit(false)}>
            Submit for review
          </button>
          <button className={btnSecondary} disabled={busy || !name} onClick={() => submit(true)}>
            Save draft
          </button>
        </div>
      </div>
    </div>
  );
}

export default function NewProposalPage() {
  return (
    <Suspense fallback={null}>
      <NewResourceForm />
    </Suspense>
  );
}
