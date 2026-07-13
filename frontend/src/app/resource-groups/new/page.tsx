"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card, btn, btnSecondary, input, label } from "@/components/ui";
import { EntityContactsEditor, fromEntityContacts, toEntityContacts, ContactRow } from "@/components/EntityContactsEditor";

function NewRGForm() {
  const router = useRouter();
  const params = useSearchParams();
  const editName = params.get("edit");
  const [name, setName] = useState(params.get("name") ?? editName ?? "");
  const [site, setSite] = useState(params.get("site") ?? "");
  const [production, setProduction] = useState(true);
  const [supportCenter, setSupportCenter] = useState("");
  const [description, setDescription] = useState("");
  const [contacts, setContacts] = useState<ContactRow[]>([]);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const { data: sites } = useQuery({ queryKey: ["sites", false], queryFn: () => api.sites() });
  const { data: editData } = useQuery({
    queryKey: ["rg-detail", editName],
    queryFn: () => api.resourceGroupDetail(editName!),
    enabled: !!editName,
  });
  useEffect(() => {
    if (!editData) return;
    setName(editData.name);
    setSite(editData.site);
    setProduction(editData.production !== false);
    setSupportCenter(editData.support_center);
    setDescription(editData.group_description);
    setContacts(fromEntityContacts(editData.contacts));
  }, [editData]);

  const submit = async (asDraft: boolean) => {
    setErr("");
    setBusy(true);
    try {
      const res = await api.proposals.create({
        entity_kind: "resource_group",
        operation: editName ? "update" : "create",
        target_name: editName ?? undefined,
        submit: !asDraft,
        proposed_state: {
          name,
          site,
          production,
          support_center: supportCenter,
          group_description: description,
          contacts: toEntityContacts(contacts),
        },
      });
      // If launched as a popout (?return=resource), send the user back to the
      // resource form with this RG preselected.
      const ret = params.get("return");
      if (ret === "resource") {
        router.push(`/proposals/new?rg=${encodeURIComponent(name)}`);
      } else {
        router.push(`/proposals/view?id=${res.id}`);
      }
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="p-8">
      <PageHeader title={editName ? `Edit resource group: ${editName}` : "New resource group"} />
      <Card className="max-w-xl">
        <div className="space-y-4">
          <div>
            <label className={label}>Name</label>
            <input className={input} value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div>
            <label className={label}>Site</label>
            <input
              className={input}
              list="site-options"
              value={site}
              onChange={(e) => setSite(e.target.value)}
              placeholder="Search sites…"
            />
            <datalist id="site-options">
              {(sites ?? []).map((s) => (
                <option key={s.name} value={s.name} />
              ))}
            </datalist>
          </div>
          <div>
            <label className={label}>Support center</label>
            <input
              className={input}
              value={supportCenter}
              onChange={(e) => setSupportCenter(e.target.value)}
            />
          </div>
          <div>
            <label className={label}>Description</label>
            <textarea
              className={input}
              rows={3}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
          <label className="flex items-center gap-2 text-sm text-gray-700">
            <input
              type="checkbox"
              checked={production}
              onChange={(e) => setProduction(e.target.checked)}
            />
            Production (vs ITB)
          </label>
        </div>
      </Card>

      <div className="mt-6">
        <EntityContactsEditor rows={contacts} onChange={setContacts} />
      </div>

      {err && <p className="mt-4 text-sm text-red-600">{err}</p>}
      <div className="mt-4 flex gap-3">
        <button className={btn} disabled={busy || !name || !site} onClick={() => submit(false)}>
          Submit for review
        </button>
        <button className={btnSecondary} disabled={busy || !name} onClick={() => submit(true)}>
          Save draft
        </button>
      </div>
    </div>
  );
}

export default function NewResourceGroupPage() {
  return (
    <Suspense fallback={null}>
      <NewRGForm />
    </Suspense>
  );
}
