"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import { PageHeader, LinkButton, Card, input } from "@/components/ui";
import { InactiveToggle } from "@/components/BrowseControls";
import { useIsReviewer, ProposeDeleteButton } from "@/components/entityActions";

export default function ResourceGroupsPage() {
  const [inactive, setInactive] = useState(false);
  const [q, setQ] = useState("");
  const isReviewer = useIsReviewer();
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({
    queryKey: ["resource-groups", inactive],
    queryFn: () => api.resourceGroups(inactive),
  });

  const rows = (data ?? []).filter(
    (r) =>
      (r.name ?? "").toLowerCase().includes(q.toLowerCase()) ||
      (r.site ?? "").toLowerCase().includes(q.toLowerCase()),
  );

  return (
    <div className="p-8">
      <PageHeader
        title="Resource groups"
        description="A resource group bundles related resources at a site under one administrative unit (production or ITB), with a support center and description. Resources always belong to a resource group."
        action={<LinkButton href="/resource-groups/new">New resource group</LinkButton>}
      />
      <div className="mb-4 flex items-center gap-4">
        <input
          className={`${input} max-w-md`}
          placeholder="Search by name or site…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <InactiveToggle value={inactive} onChange={setInactive} />
      </div>
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : rows.length === 0 ? (
        <Card>
          <p className="text-sm text-gray-500">No resource groups.</p>
        </Card>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white">
          <table className="min-w-full text-sm">
            <thead className="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500">
              <tr>
                <th className="px-4 py-2">Name</th>
                <th className="px-4 py-2">Site</th>
                <th className="px-4 py-2">Facility</th>
                <th className="px-4 py-2">Resources</th>
                <th className="px-4 py-2">Production</th>
                {isReviewer && <th className="px-4 py-2"></th>}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {rows.map((r) => (
                <tr key={r.name} className={r.deleted ? "opacity-50" : ""}>
                  <td className="px-4 py-2 font-medium">
                    <Link
                      href={`/resource-groups/detail?name=${encodeURIComponent(r.name)}`}
                      className="text-brand-700 hover:underline"
                    >
                      {r.name}
                    </Link>
                    {r.deleted && <span className="ml-2 text-xs text-red-500">(inactive)</span>}
                  </td>
                  <td className="px-4 py-2 text-gray-600">{r.site}</td>
                  <td className="px-4 py-2 text-gray-500">{r.facility}</td>
                  <td className="px-4 py-2 text-gray-500">{r.resource_count}</td>
                  <td className="px-4 py-2 text-gray-500">{r.production ? "yes" : "no"}</td>
                  {isReviewer && (
                    <td className="px-4 py-2 text-right">
                      {!r.deleted && (
                        <ProposeDeleteButton
                          entityKind="resource_group"
                          name={r.name}
                          onDone={() => qc.invalidateQueries({ queryKey: ["resource-groups"] })}
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
