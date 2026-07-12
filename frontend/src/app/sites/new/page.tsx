"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card, btn, btnSecondary, input, label } from "@/components/ui";

function NewSiteForm() {
  const router = useRouter();
  const editName = useSearchParams().get("edit");
  const [f, setF] = useState({
    name: "",
    facility: "",
    long_name: "",
    description: "",
    address_line1: "",
    city: "",
    state: "",
    country: "",
    zipcode: "",
    latitude: "",
    longitude: "",
  });
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const { data: facilities } = useQuery({
    queryKey: ["facilities", false],
    queryFn: () => api.facilities(),
  });
  const { data: editData } = useQuery({
    queryKey: ["site-detail", editName],
    queryFn: () => api.siteDetail(editName!),
    enabled: !!editName,
  });
  useEffect(() => {
    if (!editData) return;
    setF({
      name: editData.name,
      facility: editData.facility,
      long_name: editData.long_name,
      description: editData.description,
      address_line1: editData.address_line1,
      city: editData.city,
      state: editData.state,
      country: editData.country,
      zipcode: editData.zipcode,
      latitude: editData.latitude != null ? String(editData.latitude) : "",
      longitude: editData.longitude != null ? String(editData.longitude) : "",
    });
  }, [editData]);

  const set = (k: keyof typeof f) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
    setF({ ...f, [k]: e.target.value });

  const submit = async (asDraft: boolean) => {
    setErr("");
    setBusy(true);
    try {
      const proposed: Record<string, unknown> = { name: f.name, facility: f.facility };
      for (const k of ["long_name", "description", "address_line1", "city", "state", "country", "zipcode"] as const) {
        if (f[k]) proposed[k] = f[k];
      }
      if (f.latitude) proposed.latitude = Number(f.latitude);
      if (f.longitude) proposed.longitude = Number(f.longitude);
      const res = await api.proposals.create({
        entity_kind: "site",
        operation: editName ? "update" : "create",
        target_name: editName ?? undefined,
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
      <PageHeader title={editName ? `Edit site: ${editName}` : "New site"} />
      <Card className="max-w-xl">
        <div className="space-y-4">
          <div>
            <label className={label}>Name</label>
            <input className={input} value={f.name} onChange={set("name")} />
          </div>
          <div>
            <label className={label}>Facility</label>
            <input className={input} list="fac-options" value={f.facility} onChange={set("facility")} placeholder="Search facilities…" />
            <datalist id="fac-options">
              {(facilities ?? []).map((x) => (
                <option key={x.name} value={x.name} />
              ))}
            </datalist>
          </div>
          <div>
            <label className={label}>Long name</label>
            <input className={input} value={f.long_name} onChange={set("long_name")} />
          </div>
          <div>
            <label className={label}>Description</label>
            <textarea className={input} rows={2} value={f.description} onChange={set("description")} />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div><label className={label}>City</label><input className={input} value={f.city} onChange={set("city")} /></div>
            <div><label className={label}>State</label><input className={input} value={f.state} onChange={set("state")} /></div>
            <div><label className={label}>Country</label><input className={input} value={f.country} onChange={set("country")} /></div>
            <div><label className={label}>Zipcode</label><input className={input} value={f.zipcode} onChange={set("zipcode")} /></div>
            <div><label className={label}>Latitude</label><input className={input} value={f.latitude} onChange={set("latitude")} /></div>
            <div><label className={label}>Longitude</label><input className={input} value={f.longitude} onChange={set("longitude")} /></div>
          </div>
          {err && <p className="text-sm text-red-600">{err}</p>}
          <div className="flex gap-3 pt-2">
            <button className={btn} disabled={busy || !f.name || !f.facility} onClick={() => submit(false)}>Submit for review</button>
            <button className={btnSecondary} disabled={busy || !f.name} onClick={() => submit(true)}>Save draft</button>
          </div>
        </div>
      </Card>
    </div>
  );
}

export default function NewSitePage() {
  return (
    <Suspense fallback={null}>
      <NewSiteForm />
    </Suspense>
  );
}
