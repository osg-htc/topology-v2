"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import { PageHeader, LinkButton, Card, input } from "@/components/ui";
import { InactiveToggle } from "@/components/BrowseControls";
import { useIsReviewer, ProposeDeleteButton } from "@/components/entityActions";

export default function ProjectsPage() {
  const [inactive, setInactive] = useState(false);
  const [q, setQ] = useState("");
  const isReviewer = useIsReviewer();
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({
    queryKey: ["projects", inactive],
    queryFn: () => api.projects(inactive),
  });

  const rows = (data ?? []).filter(
    (r) =>
      (r.name ?? "").toLowerCase().includes(q.toLowerCase()) ||
      (r.organization ?? "").toLowerCase().includes(q.toLowerCase()) ||
      (r.pi_name ?? "").toLowerCase().includes(q.toLowerCase()),
  );

  return (
    <div className="p-8">
      <PageHeader
        title="Projects"
        description="Projects describe the science and PIs that use OSG resources. Each has a field of science, an organization/PI, an institution, and a sponsor (a campus grid or a VO)."
        action={<LinkButton href="/projects/new">New project</LinkButton>}
      />
      <div className="mb-4 flex items-center gap-4">
        <input
          className={`${input} max-w-md`}
          placeholder="Search by name, organization, or PI…"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <InactiveToggle value={inactive} onChange={setInactive} />
      </div>
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : rows.length === 0 ? (
        <Card>
          <p className="text-sm text-gray-500">No projects.</p>
        </Card>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white">
          <table className="min-w-full text-sm">
            <thead className="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500">
              <tr>
                <th className="px-4 py-2">Name</th>
                <th className="px-4 py-2">PI</th>
                <th className="px-4 py-2">Organization</th>
                <th className="px-4 py-2">Field of science</th>
                <th className="px-4 py-2">Sponsor</th>
                {isReviewer && <th className="px-4 py-2"></th>}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {rows.slice(0, 500).map((r) => (
                <tr key={r.name} className={r.deleted ? "opacity-50" : ""}>
                  <td className="px-4 py-2 font-medium">
                    <Link
                      href={`/projects/detail?name=${encodeURIComponent(r.name)}`}
                      className="text-brand-700 hover:underline"
                    >
                      {r.name}
                    </Link>
                    {r.deleted && <span className="ml-2 text-xs text-red-500">(inactive)</span>}
                  </td>
                  <td className="px-4 py-2 text-gray-600">{r.pi_name}</td>
                  <td className="px-4 py-2 text-gray-500">{r.organization}</td>
                  <td className="px-4 py-2 text-gray-500">{r.field_of_science}</td>
                  <td className="px-4 py-2 text-gray-500">
                    {r.sponsor_name ? `${r.sponsor_name}` : "—"}
                  </td>
                  {isReviewer && (
                    <td className="px-4 py-2 text-right">
                      {!r.deleted && (
                        <ProposeDeleteButton
                          entityKind="project"
                          name={r.name}
                          onDone={() => qc.invalidateQueries({ queryKey: ["projects"] })}
                        />
                      )}
                    </td>
                  )}
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
