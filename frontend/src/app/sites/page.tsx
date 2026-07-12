"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { PageHeader, LinkButton, Card, input } from "@/components/ui";
import { InactiveToggle } from "@/components/BrowseControls";
import { useIsReviewer, ProposeDeleteButton } from "@/components/entityActions";

export default function SitesPage() {
  const [inactive, setInactive] = useState(false);
  const [q, setQ] = useState("");
  const isReviewer = useIsReviewer();
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({
    queryKey: ["sites", inactive],
    queryFn: () => api.sites(inactive),
  });

  const rows = (data ?? []).filter((r) =>
    (r.name ?? "").toLowerCase().includes(q.toLowerCase()),
  );

  return (
    <div className="p-8">
      <PageHeader title="Sites" action={<LinkButton href="/sites/new">New site</LinkButton>} />
      <div className="mb-4 flex items-center gap-4">
        <input
          className={`${input} max-w-md`}
          placeholder="Search by name…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <InactiveToggle value={inactive} onChange={setInactive} />
      </div>
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : rows.length === 0 ? (
        <Card>
          <p className="text-sm text-gray-500">No sites.</p>
        </Card>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white">
          <table className="min-w-full text-sm">
            <thead className="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500">
              <tr>
                <th className="px-4 py-2">Name</th>
                <th className="px-4 py-2">Facility</th>
                <th className="px-4 py-2">City</th>
                <th className="px-4 py-2">Country</th>
                {isReviewer && <th className="px-4 py-2"></th>}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {rows.map((r) => (
                <tr key={r.name} className={r.deleted ? "opacity-50" : ""}>
                  <td className="px-4 py-2 font-medium text-navy-900">
                    {r.name}
                    {r.deleted && <span className="ml-2 text-xs text-red-500">(inactive)</span>}
                  </td>
                  <td className="px-4 py-2 text-gray-600">{r.facility}</td>
                  <td className="px-4 py-2 text-gray-500">{r.city}</td>
                  <td className="px-4 py-2 text-gray-500">{r.country}</td>
                  {isReviewer && (
                    <td className="px-4 py-2 text-right">
                      {!r.deleted && (
                        <ProposeDeleteButton
                          entityKind="site"
                          name={r.name}
                          onDone={() => qc.invalidateQueries({ queryKey: ["sites"] })}
                        />
                      )}
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
