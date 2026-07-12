"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card, btn, btnSecondary, input, label } from "@/components/ui";

export default function NewFacilityPage() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [institutionID, setInstitutionID] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const { data: institutions } = useQuery({ queryKey: ["institutions"], queryFn: api.institutions });

  const submit = async (asDraft: boolean) => {
    setErr("");
    setBusy(true);
    try {
      const res = await api.proposals.create({
        entity_kind: "facility",
        operation: "create",
        submit: !asDraft,
        proposed_state: { name, institution_id: institutionID },
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
      <PageHeader title="New facility" />
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
          {err && <p className="text-sm text-red-600">{err}</p>}
          <div className="flex gap-3 pt-2">
            <button className={btn} disabled={busy || !name} onClick={() => submit(false)}>
              Submit for review
            </button>
            <button className={btnSecondary} disabled={busy || !name} onClick={() => submit(true)}>
              Save draft
            </button>
          </div>
        </div>
      </Card>
    </div>
  );
}
