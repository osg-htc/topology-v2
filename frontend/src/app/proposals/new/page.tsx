"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card, btn, btnSecondary, input, label } from "@/components/ui";
import { MultiSelect } from "@/components/MultiSelect";

const CONTACT_TYPES = [
  "Administrative Contact",
  "Security Contact",
  "Executive Contact",
  "Local Operational Contact",
  "Local Security Contact",
];
const RANKS = ["Primary", "Secondary", "Tertiary"];
const COMMON_TAGS = ["OSPool", "CC*"];

type ContactRow = { type: string; rank: string; name: string; id: string };
type ServiceRow = { name: string; description: string };

function NewResourceForm() {
  const router = useRouter();
  const params = useSearchParams();
  const [name, setName] = useState("");
  const [rg, setRg] = useState(params.get("rg") ?? "");
  const [hostname, setHostname] = useState("");
  const [description, setDescription] = useState("");
  const [active, setActive] = useState(false);
  const [aliases, setAliases] = useState<string[]>([]);
  const [tags, setTags] = useState<string[]>([]);
  const [allowedVOs, setAllowedVOs] = useState<string[]>([]);
  const [contacts, setContacts] = useState<ContactRow[]>([
    { type: "Administrative Contact", rank: "Primary", name: "", id: "" },
    { type: "Security Contact", rank: "Primary", name: "", id: "" },
  ]);
  const [services, setServices] = useState<ServiceRow[]>([{ name: "", description: "" }]);
  const [advanced, setAdvanced] = useState(false);
  const [advancedJSON, setAdvancedJSON] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const { data: rgs } = useQuery({ queryKey: ["resource-groups", false], queryFn: () => api.resourceGroups() });
  const { data: serviceNames } = useQuery({ queryKey: ["service-names"], queryFn: api.serviceNames });
  const { data: voNames } = useQuery({ queryKey: ["vo-names"], queryFn: api.voNames });
  const { data: tagNames } = useQuery({ queryKey: ["tag-names"], queryFn: api.tagNames });
  const { data: knownContacts } = useQuery({ queryKey: ["contacts"], queryFn: api.contacts });

  const rgValid = new Set((rgs ?? []).map((g) => g.name)).has(rg);
  const hasContact = contacts.some((c) => c.name || c.id);
  const tagOptions = Array.from(new Set([...COMMON_TAGS, ...(tagNames ?? [])]));

  const buildResource = (): Record<string, unknown> => {
    const resource: Record<string, unknown> = { FQDN: hostname, Active: active };
    if (description) resource.Description = description;
    if (aliases.length) resource.FQDNAliases = aliases;
    if (tags.length) resource.Tags = tags;
    if (allowedVOs.length) resource.AllowedVOs = allowedVOs;

    const svc: Record<string, unknown> = {};
    for (const s of services) if (s.name) svc[s.name] = s.description ? { Description: s.description } : {};
    if (Object.keys(svc).length) resource.Services = svc;

    const cl: Record<string, Record<string, unknown>> = {};
    for (const c of contacts) {
      if (!c.name && !c.id) continue;
      cl[c.type] = cl[c.type] || {};
      cl[c.type][c.rank] = { Name: c.name, ID: c.id };
    }
    if (Object.keys(cl).length) resource.ContactLists = cl;
    return resource;
  };

  const toggleAdvanced = () => {
    if (!advanced) setAdvancedJSON(JSON.stringify(buildResource(), null, 2));
    setAdvanced(!advanced);
  };

  const submit = async (asDraft: boolean) => {
    setErr("");
    setBusy(true);
    try {
      let resource: unknown;
      if (advanced) {
        resource = JSON.parse(advancedJSON);
      } else {
        resource = buildResource();
      }
      const res = await api.proposals.create({
        entity_kind: "resource",
        operation: "create",
        submit: !asDraft,
        proposed_state: { name, resource_group: rg, resource },
      });
      router.push(`/proposals/view?id=${res.id}`);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  const setContact = (i: number, patch: Partial<ContactRow>) =>
    setContacts(contacts.map((c, j) => (j === i ? { ...c, ...patch } : c)));
  const setService = (i: number, patch: Partial<ServiceRow>) =>
    setServices(services.map((s, j) => (j === i ? { ...s, ...patch } : s)));
  const pickContact = (i: number, nameVal: string) => {
    const match = (knownContacts ?? []).find((c) => c.name === nameVal);
    setContact(i, { name: nameVal, ...(match ? { id: match.id } : {}) });
  };

  return (
    <div className="p-8">
      <PageHeader
        title="Register a resource"
        description="Provide the resource's placement, host name, services, and contacts. At least one contact is required for a complete registration."
      />
      <div className="max-w-2xl space-y-6">
        <Card>
          <h3 className="mb-3 text-sm font-semibold text-gray-700">Basics</h3>
          <div className="space-y-4">
            <div>
              <label className={label}>Resource name</label>
              <input className={input} value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. UChicago_OSGConnect_ap20" />
            </div>
            <div>
              <div className="mb-1 flex items-center justify-between">
                <label className={`${label} mb-0`}>Resource group</label>
                <Link href="/resource-groups/new?return=resource" className="text-xs text-brand-600 hover:underline">
                  + New resource group
                </Link>
              </div>
              <input className={input} list="rg-options" value={rg} onChange={(e) => setRg(e.target.value)} placeholder="Search resource groups…" />
              <datalist id="rg-options">
                {(rgs ?? []).map((g) => (
                  <option key={g.name} value={g.name}>{g.site} · {g.facility}</option>
                ))}
              </datalist>
              {rg && !rgValid && (
                <p className="mt-1 text-xs text-amber-600">“{rg}” is not an existing resource group — pick one or create it.</p>
              )}
            </div>
            <div>
              <label className={label}>Host name</label>
              <input className={input} value={hostname} onChange={(e) => setHostname(e.target.value)} placeholder="host.example.org" />
            </div>
            <div>
              <label className={label}>Host name aliases</label>
              <MultiSelect options={[]} value={aliases} onChange={setAliases} placeholder="Add an alias and press Enter…" />
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
              className="text-xs text-brand-600 hover:underline"
              onClick={() => setContacts([...contacts, { type: "Administrative Contact", rank: "Primary", name: "", id: "" }])}
            >
              + Add contact
            </button>
          </div>
          {!hasContact && <p className="mb-2 text-xs text-amber-600">At least one contact is required.</p>}
          <div className="space-y-2">
            {contacts.map((c, i) => (
              <div key={i} className="grid grid-cols-4 gap-2">
                <select className={input} value={c.type} onChange={(e) => setContact(i, { type: e.target.value })}>
                  {CONTACT_TYPES.map((t) => <option key={t} value={t}>{t}</option>)}
                </select>
                <select className={input} value={c.rank} onChange={(e) => setContact(i, { rank: e.target.value })}>
                  {RANKS.map((rk) => <option key={rk} value={rk}>{rk}</option>)}
                </select>
                <input
                  className={input}
                  list="contact-options"
                  placeholder="Contact"
                  value={c.name}
                  onChange={(e) => pickContact(i, e.target.value)}
                />
                <input className={input} placeholder="ID" value={c.id} onChange={(e) => setContact(i, { id: e.target.value })} />
              </div>
            ))}
          </div>
          <datalist id="contact-options">
            {(knownContacts ?? []).slice(0, 1000).map((c) => (
              <option key={`${c.name}-${c.id}`} value={c.name}>{c.id}</option>
            ))}
          </datalist>
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
          <button className={btn} disabled={busy || !name || !rgValid || (!advanced && (!hostname || !hasContact))} onClick={() => submit(false)}>
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
