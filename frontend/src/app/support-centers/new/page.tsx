"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card, btn, btnSecondary, input, label } from "@/components/ui";

function NewSupportCenterForm() {
  const router = useRouter();
  const editName = useSearchParams().get("edit");
  const [f, setF] = useState({ name: editName ?? "", long_name: "", community: "", description: "" });
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const { data: editData } = useQuery({
    queryKey: ["support-center", editName],
    queryFn: () => api.supportCenter(editName!),
    enabled: !!editName,
  });
  useEffect(() => {
    if (!editData) return;
    setF({
      name: editData.name,
      long_name: editData.long_name,
      community: editData.community,
      description: editData.description,
    });
  }, [editData]);

  const set = (k: keyof typeof f) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
    setF({ ...f, [k]: e.target.value });

  const submit = async (asDraft: boolean) => {
    setErr("");
    setBusy(true);
    try {
      // Every field this form renders is always sent, even empty -- the
      // backend merges a submission onto the center's current state by field
      // presence. Contacts (and anything else support-centers.yaml carries)
      // has no input here and is deliberately never mentioned, so it's
      // preserved rather than wiped.
      const res = await api.proposals.create({
        entity_kind: "support_center",
        operation: editName ? "update" : "create",
        target_name: editName ?? undefined,
        submit: !asDraft,
        proposed_state: { name: f.name.trim(), long_name: f.long_name, community: f.community, description: f.description },
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
      <PageHeader title={editName ? `Edit support center: ${editName}` : "New support center"} />
      <Card className="max-w-xl">
        <div className="space-y-4">
          <div>
            <label className={label}>Name</label>
            <input className={input} value={f.name} onChange={set("name")} />
            {editName && (
              <p className="mt-1 text-xs text-gray-400">
                Renaming also updates every resource group that uses this support center. Its ID never changes.
              </p>
            )}
          </div>
          <div>
            <label className={label}>Long name</label>
            <input className={input} value={f.long_name} onChange={set("long_name")} />
          </div>
          <div>
            <label className={label}>Community</label>
            <input className={input} value={f.community} onChange={set("community")} />
          </div>
          <div>
            <label className={label}>Description</label>
            <textarea className={input} rows={3} value={f.description} onChange={set("description")} />
          </div>
        </div>
      </Card>
      {err && <p className="mt-4 text-sm text-red-600">{err}</p>}
      <div className="mt-4 flex gap-3">
        <button className={btn} disabled={busy || !f.name.trim()} onClick={() => submit(false)}>Submit for review</button>
        <button className={btnSecondary} disabled={busy || !f.name.trim()} onClick={() => submit(true)}>Save draft</button>
      </div>
    </div>
  );
}

export default function NewSupportCenterPage() {
  return (
    <Suspense fallback={null}>
      <NewSupportCenterForm />
    </Suspense>
  );
}
