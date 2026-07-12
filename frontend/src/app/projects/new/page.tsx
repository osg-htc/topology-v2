"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card, btn, btnSecondary, input, label } from "@/components/ui";

export default function NewProjectPage() {
  const router = useRouter();
  const [f, setF] = useState({
    name: "",
    id: "",
    description: "",
    department: "",
    field_of_science: "",
    field_of_science_id: "",
    organization: "",
    pi_name: "",
    institution_id: "",
    sponsor_type: "CampusGrid",
    sponsor_name: "",
  });
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const set = (k: keyof typeof f) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>) =>
    setF({ ...f, [k]: e.target.value });

  const submit = async (asDraft: boolean) => {
    setErr("");
    setBusy(true);
    try {
      const proposed: Record<string, unknown> = { name: f.name };
      for (const k of ["id", "description", "department", "field_of_science", "field_of_science_id", "organization", "pi_name", "institution_id"] as const) {
        if (f[k]) proposed[k] = f[k];
      }
      if (f.sponsor_name) {
        proposed.sponsor = { [f.sponsor_type]: { Name: f.sponsor_name } };
      }
      const res = await api.proposals.create({
        entity_kind: "project",
        operation: "create",
        submit: !asDraft,
        proposed_state: proposed,
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
      <PageHeader title="New project" />
      <Card className="max-w-xl">
        <div className="space-y-4">
          <div><label className={label}>Name</label><input className={input} value={f.name} onChange={set("name")} /></div>
          <div><label className={label}>PI name</label><input className={input} value={f.pi_name} onChange={set("pi_name")} /></div>
          <div><label className={label}>Organization</label><input className={input} value={f.organization} onChange={set("organization")} /></div>
          <div><label className={label}>Department</label><input className={input} value={f.department} onChange={set("department")} /></div>
          <div><label className={label}>Field of science</label><input className={input} value={f.field_of_science} onChange={set("field_of_science")} /></div>
          <div><label className={label}>Field of science ID</label><input className={input} value={f.field_of_science_id} onChange={set("field_of_science_id")} /></div>
          <div><label className={label}>Institution ID (OSG IID)</label><input className={input} value={f.institution_id} onChange={set("institution_id")} /></div>
          <div>
            <label className={label}>Description</label>
            <textarea className={input} rows={3} value={f.description} onChange={set("description")} />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className={label}>Sponsor type</label>
              <select className={input} value={f.sponsor_type} onChange={set("sponsor_type")}>
                <option value="CampusGrid">CampusGrid</option>
                <option value="VirtualOrganization">VirtualOrganization</option>
              </select>
            </div>
            <div>
              <label className={label}>Sponsor name</label>
              <input className={input} value={f.sponsor_name} onChange={set("sponsor_name")} />
            </div>
          </div>
          {err && <p className="text-sm text-red-600">{err}</p>}
          <div className="flex gap-3 pt-2">
            <button className={btn} disabled={busy || !f.name} onClick={() => submit(false)}>Submit for review</button>
            <button className={btnSecondary} disabled={busy || !f.name} onClick={() => submit(true)}>Save draft</button>
          </div>
        </div>
      </Card>
    </div>
  );
}
