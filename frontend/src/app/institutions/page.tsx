"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { PageHeader, Card, input, btn } from "@/components/ui";

// Institutions come from the external OSG registry (OSG IID <-> ROR). They are
// read-only here; administrators can refresh the local cache.
export default function InstitutionsPage() {
  const [q, setQ] = useState("");
  const [msg, setMsg] = useState("");
  const qc = useQueryClient();
  const { data: session } = useQuery({ queryKey: ["me"], queryFn: api.auth.me, retry: false });
  const isAdmin = session?.effective_role === "administrator";

  const { data, isLoading } = useQuery({ queryKey: ["institutions"], queryFn: api.institutions });

  const sync = useMutation({
    mutationFn: api.admin.syncInstitutions,
    onSuccess: (r) => {
      setMsg(`Synced ${r.synced} institutions.`);
      qc.invalidateQueries({ queryKey: ["institutions"] });
    },
    onError: (e) => setMsg(String(e)),
  });

  const rows = (data ?? []).filter(
    (r) =>
      (r.name ?? "").toLowerCase().includes(q.toLowerCase()) ||
      (r.id ?? "").toLowerCase().includes(q.toLowerCase()),
  );

  return (
    <div className="p-8">
      <PageHeader
        title="Institutions"
        action={
          isAdmin ? (
            <button className={btn} disabled={sync.isPending} onClick={() => sync.mutate()}>
              Sync from registry
            </button>
          ) : undefined
        }
      />
      {msg && <p className="mb-3 text-sm text-gray-600">{msg}</p>}
      <input
        className={`${input} mb-4 max-w-md`}
        placeholder="Search by name or IID…"
        value={q}
        onChange={(e) => setQ(e.target.value)}
      />
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : rows.length === 0 ? (
        <Card>
          <p className="text-sm text-gray-500">
            No institutions cached yet.
            {isAdmin && " Use “Sync from registry” to populate."}
          </p>
        </Card>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white">
          <table className="min-w-full text-sm">
            <thead className="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500">
              <tr>
                <th className="px-4 py-2">Name</th>
                <th className="px-4 py-2">OSG IID</th>
                <th className="px-4 py-2">ROR</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {rows.slice(0, 500).map((r) => (
                <tr key={r.id}>
                  <td className="px-4 py-2 font-medium text-navy-900">{r.name}</td>
                  <td className="px-4 py-2 text-xs text-gray-500">{r.id}</td>
                  <td className="px-4 py-2 text-xs text-gray-500">{r.ror_id || "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {rows.length > 500 && (
            <p className="p-3 text-xs text-gray-400">Showing first 500 of {rows.length}.</p>
          )}
        </div>
      )}
    </div>
  );
}
