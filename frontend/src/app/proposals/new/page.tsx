"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card, btn, btnSecondary, input, label } from "@/components/ui";

// Register-a-resource form. Submits a "create resource" change proposal, which a
// manager/administrator then reviews and approves.
export default function NewProposalPage() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [rg, setRg] = useState("");
  const [fqdn, setFqdn] = useState("");
  const [description, setDescription] = useState("");
  const [active, setActive] = useState(false);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  const submit = async (asDraft: boolean) => {
    setErr("");
    setBusy(true);
    try {
      const res = await api.proposals.create({
        entity_kind: "resource",
        operation: "create",
        submit: !asDraft,
        proposed_state: {
          name,
          resource_group: rg,
          resource: { FQDN: fqdn, Active: active, Description: description },
        },
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
      <PageHeader title="Register a resource" />
      <Card className="max-w-xl">
        <div className="space-y-4">
          <div>
            <label className={label}>Resource name</label>
            <input className={input} value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div>
            <label className={label}>Resource group</label>
            <input className={input} value={rg} onChange={(e) => setRg(e.target.value)} />
          </div>
          <div>
            <label className={label}>FQDN</label>
            <input className={input} value={fqdn} onChange={(e) => setFqdn(e.target.value)} />
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
            <input type="checkbox" checked={active} onChange={(e) => setActive(e.target.checked)} />
            Active (accepting requests)
          </label>
          {err && <p className="text-sm text-red-600">{err}</p>}
          <div className="flex gap-3 pt-2">
            <button className={btn} disabled={busy || !name || !rg || !fqdn} onClick={() => submit(false)}>
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
