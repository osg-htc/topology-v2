"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card, btn, btnSecondary, input, label } from "@/components/ui";

const CONTACT_TYPES = [
  "Administrative Contact",
  "Security Contact",
  "Executive Contact",
  "Local Operational Contact",
  "Local Security Contact",
];
const RANKS = ["Primary", "Secondary", "Tertiary"];

type ContactRow = { type: string; rank: string; name: string; id: string };
type ServiceRow = { name: string; description: string };

function csvToArray(s: string): string[] {
  return s
    .split(",")
    .map((x) => x.trim())
    .filter(Boolean);
}

function NewResourceForm() {
  const router = useRouter();
  const params = useSearchParams();
  const [name, setName] = useState("");
  const [rg, setRg] = useState(params.get("rg") ?? "");
  const [fqdn, setFqdn] = useState("");
  const [description, setDescription] = useState("");
  const [active, setActive] = useState(false);
  const [dn, setDn] = useState("");
  const [aliases, setAliases] = useState("");
  const [tags, setTags] = useState("");
  const [allowedVOs, setAllowedVOs] = useState("");
  const [contacts, setContacts] = useState<ContactRow[]>([
    { type: "Administrative Contact", rank: "Primary", name: "", id: "" },
    { type: "Security Contact", rank: "Primary", name: "", id: "" },
  ]);
  const [services, setServices] = useState<ServiceRow[]>([{ name: "", description: "" }]);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const { data: rgs } = useQuery({
    queryKey: ["resource-groups", false],
    queryFn: () => api.resourceGroups(),
  });
  const rgValid = new Set((rgs ?? []).map((g) => g.name)).has(rg);
  const hasContact = contacts.some((c) => c.name || c.id);

  const buildResource = () => {
    const resource: Record<string, unknown> = { FQDN: fqdn, Active: active };
    if (description) resource.Description = description;
    if (dn) resource.DN = dn;
    if (csvToArray(aliases).length) resource.FQDNAliases = csvToArray(aliases);
    if (csvToArray(tags).length) resource.Tags = csvToArray(tags);
    if (csvToArray(allowedVOs).length) resource.AllowedVOs = csvToArray(allowedVOs);

    const svc: Record<string, unknown> = {};
    for (const s of services) {
      if (s.name) svc[s.name] = s.description ? { Description: s.description } : {};
    }
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

  const submit = async (asDraft: boolean) => {
    setErr("");
    setBusy(true);
    try {
      const res = await api.proposals.create({
        entity_kind: "resource",
        operation: "create",
        submit: !asDraft,
        proposed_state: { name, resource_group: rg, resource: buildResource() },
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

  return (
    <div className="p-8">
      <PageHeader
        title="Register a resource"
        description="Provide the resource's placement, network identity, services, and contacts. At least one contact is required for a complete registration."
      />
      <div className="max-w-2xl space-y-6">
        <Card>
          <h3 className="mb-3 text-sm font-semibold text-gray-700">Basics</h3>
          <div className="space-y-4">
            <div>
              <label className={label}>Resource name</label>
              <input
                className={input}
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. UChicago_OSGConnect_ap20"
              />
            </div>
            <div>
              <div className="mb-1 flex items-center justify-between">
                <label className={`${label} mb-0`}>Resource group</label>
                <Link href="/resource-groups/new?return=resource" className="text-xs text-brand-600 hover:underline">
                  + New resource group
                </Link>
              </div>
              <input
                className={input}
                list="rg-options"
                value={rg}
                onChange={(e) => setRg(e.target.value)}
                placeholder="Search resource groups…"
              />
              <datalist id="rg-options">
                {(rgs ?? []).map((g) => (
                  <option key={g.name} value={g.name}>
                    {g.site} · {g.facility}
                  </option>
                ))}
              </datalist>
              {rg && !rgValid && (
                <p className="mt-1 text-xs text-amber-600">
                  “{rg}” is not an existing resource group — pick one or create it.
                </p>
              )}
            </div>
            <div>
              <label className={label}>FQDN</label>
              <input
                className={input}
                value={fqdn}
                onChange={(e) => setFqdn(e.target.value)}
                placeholder="host.example.org"
              />
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
          <h3 className="mb-3 text-sm font-semibold text-gray-700">Network &amp; tags</h3>
          <div className="space-y-4">
            <div>
              <label className={label}>FQDN aliases (comma-separated)</label>
              <input className={input} value={aliases} onChange={(e) => setAliases(e.target.value)} />
            </div>
            <div>
              <label className={label}>DN (host certificate — required for XCache)</label>
              <input className={input} value={dn} onChange={(e) => setDn(e.target.value)} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className={label}>Tags (comma-separated)</label>
                <input className={input} value={tags} onChange={(e) => setTags(e.target.value)} placeholder="OSPool, CC*" />
              </div>
              <div>
                <label className={label}>Allowed VOs (comma-separated)</label>
                <input className={input} value={allowedVOs} onChange={(e) => setAllowedVOs(e.target.value)} />
              </div>
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
                <input className={input} placeholder="Service name (e.g. CE)" value={s.name} onChange={(e) => setService(i, { name: e.target.value })} />
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
          {!hasContact && (
            <p className="mb-2 text-xs text-amber-600">At least one contact is required.</p>
          )}
          <div className="space-y-2">
            {contacts.map((c, i) => (
              <div key={i} className="grid grid-cols-4 gap-2">
                <select className={input} value={c.type} onChange={(e) => setContact(i, { type: e.target.value })}>
                  {CONTACT_TYPES.map((t) => (
                    <option key={t} value={t}>{t}</option>
                  ))}
                </select>
                <select className={input} value={c.rank} onChange={(e) => setContact(i, { rank: e.target.value })}>
                  {RANKS.map((rk) => (
                    <option key={rk} value={rk}>{rk}</option>
                  ))}
                </select>
                <input className={input} placeholder="Name" value={c.name} onChange={(e) => setContact(i, { name: e.target.value })} />
                <input className={input} placeholder="Contact ID (OSG… or email hash)" value={c.id} onChange={(e) => setContact(i, { id: e.target.value })} />
              </div>
            ))}
          </div>
        </Card>

        {err && <p className="text-sm text-red-600">{err}</p>}
        <div className="flex gap-3">
          <button
            className={btn}
            disabled={busy || !name || !rgValid || !fqdn || !hasContact}
            onClick={() => submit(false)}
          >
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
