"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card, btn, btnSecondary, input, label } from "@/components/ui";
import { EntityContactsEditor, fromEntityContacts, toEntityContacts, ContactRow } from "@/components/EntityContactsEditor";

function NewFacilityForm() {
  const router = useRouter();
  const editName = useSearchParams().get("edit");
  const [name, setName] = useState(editName ?? "");
  const [institutionID, setInstitutionID] = useState("");
  const [contacts, setContacts] = useState<ContactRow[]>([]);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const { data: institutions } = useQuery({ queryKey: ["institutions"], queryFn: api.institutions });
  const { data: editData } = useQuery({
    queryKey: ["facility-detail", editName],
    queryFn: () => api.facilityDetail(editName!),
    enabled: !!editName,
  });
  useEffect(() => {
    if (!editData) return;
    setName(editData.name);
    setInstitutionID(editData.institution_id);
    setContacts(fromEntityContacts(editData.contacts));
  }, [editData]);

  const submit = async (asDraft: boolean) => {
    setErr("");
    setBusy(true);
    try {
      const res = await api.proposals.create({
        entity_kind: "facility",
        operation: editName ? "update" : "create",
        target_name: editName ?? undefined,
        submit: !asDraft,
        proposed_state: { name, institution_id: institutionID, contacts: toEntityContacts(contacts) },
      });
      router.push(`/proposals/view?id=${res.id}`);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="p-8">
      <PageHeader title={editName ? `Edit facility: ${editName}` : "New facility"} />
      <Card className="max-w-xl">
        <div className="space-y-4">
          <div>
            <label className={label}>Name</label>
            <input className={input} value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div>
            <label className={label}>Institution</label>
            <input
              className={input}
              list="inst-options"
              value={institutionID}
              onChange={(e) => setInstitutionID(e.target.value)}
              placeholder="OSG IID (or pick a name)…"
            />
            <datalist id="inst-options">
              {(institutions ?? []).map((i) => (
                <option key={i.id} value={i.id}>
                  {i.name}
                </option>
              ))}
            </datalist>
            <p className="mt-1 text-xs text-gray-400">
              Institutions come from the registry (Institutions page → Sync).
            </p>
          </div>
        </div>
      </Card>
      <div className="mt-6 max-w-xl">
        <EntityContactsEditor rows={contacts} onChange={setContacts} />
      </div>
      {err && <p className="mt-4 text-sm text-red-600">{err}</p>}
      <div className="mt-4 flex gap-3">
        <button className={btn} disabled={busy || !name} onClick={() => submit(false)}>
          Submit for review
        </button>
        <button className={btnSecondary} disabled={busy || !name} onClick={() => submit(true)}>
          Save draft
        </button>
      </div>
    </div>
  );
}

export default function NewFacilityPage() {
  return (
    <Suspense fallback={null}>
      <NewFacilityForm />
    </Suspense>
  );
}
