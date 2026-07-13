"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api, Project } from "@/lib/api";
import { PageHeader, LinkButton, Card, input } from "@/components/ui";
import { InactiveToggle } from "@/components/BrowseControls";
import { DataTable } from "@/components/DataTable";
import { useIsReviewer } from "@/components/entityActions";

export default function ProjectsPage() {
  const [inactive, setInactive] = useState(false);
  const [q, setQ] = useState("");
  const isReviewer = useIsReviewer();
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({ queryKey: ["projects", inactive], queryFn: () => api.projects(inactive) });

  const rows = (data ?? [])
    .filter(
      (r) =>
        (r.name ?? "").toLowerCase().includes(q.toLowerCase()) ||
        (r.organization ?? "").toLowerCase().includes(q.toLowerCase()) ||
        (r.pi_name ?? "").toLowerCase().includes(q.toLowerCase()),
    )
    .slice(0, 500);

  return (
    <div className="p-8">
      <PageHeader
        title="Projects"
        description="Projects describe the science and PIs that use OSG resources — field of science, organization/PI, institution, and a sponsor (campus grid or VO). Click a row to expand; use the icons to open, edit, or delete."
        action={<LinkButton href="/projects/new">New project</LinkButton>}
      />
      <div className="mb-4 flex items-center gap-4">
        <input className={`${input} max-w-md`} placeholder="Search by name, organization, or PI…" value={q} onChange={(e) => setQ(e.target.value)} />
        <InactiveToggle value={inactive} onChange={setInactive} />
      </div>
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : rows.length === 0 ? (
        <Card><p className="text-sm text-gray-500">No projects.</p></Card>
      ) : (
        <DataTable<Project>
          rows={rows}
          rowKey={(r) => r.name}
          canDelete={isReviewer}
          columns={[
            { header: "Name", sortValue: (r) => r.name, cell: (r) => <span className="font-medium text-navy-900">{r.name}{r.deleted && <span className="ml-2 text-xs text-red-500">(inactive)</span>}</span> },
            { header: "PI", sortValue: (r) => r.pi_name, cell: (r) => <span className="text-gray-600">{r.pi_name}</span> },
            { header: "Organization", sortValue: (r) => r.organization, cell: (r) => <span className="text-gray-500">{r.organization}</span> },
            { header: "Field of science", sortValue: (r) => r.field_of_science, cell: (r) => <span className="text-gray-500">{r.field_of_science}</span> },
            { header: "Sponsor", sortValue: (r) => r.sponsor_name, cell: (r) => <span className="text-gray-500">{r.sponsor_name || "—"}</span> },
          ]}
          actions={(r) => ({
            detailHref: `/projects/detail?name=${encodeURIComponent(r.name)}`,
            editHref: `/projects/new?edit=${encodeURIComponent(r.name)}`,
            entityKind: "project",
            name: r.name,
            onChanged: () => qc.invalidateQueries({ queryKey: ["projects"] }),
          })}
        />
      )}
    </div>
  );
}
