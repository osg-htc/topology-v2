"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card, btn, btnSecondary, input, label } from "@/components/ui";

// Register-a-resource form. Submits a "create resource" change proposal, which a
// manager/administrator then reviews and approves.
function NewResourceForm() {
  const router = useRouter();
  const params = useSearchParams();
  const [name, setName] = useState("");
  const [rg, setRg] = useState(params.get("rg") ?? "");
  const [fqdn, setFqdn] = useState("");
  const [description, setDescription] = useState("");
  const [active, setActive] = useState(false);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  // Resource groups drive the dropdown; must be an existing (active) RG.
  const { data: rgs } = useQuery({
    queryKey: ["resource-groups", false],
    queryFn: () => api.resourceGroups(),
  });
  const rgNames = new Set((rgs ?? []).map((g) => g.name));
  const rgValid = rgNames.has(rg);

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
            <div className="mb-1 flex items-center justify-between">
              <label className={`${label} mb-0`}>Resource group</label>
              <Link
                href={`/resource-groups/new?return=resource`}
                className="text-xs text-brand-600 hover:underline"
              >
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
                “{rg}” is not an existing resource group. Pick one from the list or create it.
              </p>
            )}
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
            <button
              className={btn}
              disabled={busy || !name || !rgValid || !fqdn}
              onClick={() => submit(false)}
            >
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

export default function NewProposalPage() {
  return (
    <Suspense fallback={null}>
      <NewResourceForm />
    </Suspense>
  );
}
