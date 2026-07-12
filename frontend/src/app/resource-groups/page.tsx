"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Link from "next/link";
import { api, ResourceGroup } from "@/lib/api";
import { PageHeader, LinkButton, Card, input } from "@/components/ui";
import { InactiveToggle } from "@/components/BrowseControls";
import { DataTable } from "@/components/DataTable";
import { useIsReviewer } from "@/components/entityActions";

function ExpandedRG({ name }: { name: string }) {
  const { data: rg } = useQuery({ queryKey: ["rg-detail", name], queryFn: () => api.resourceGroupDetail(name) });
  if (!rg) return <p className="text-sm text-gray-400">Loading…</p>;
  return (
    <div className="text-sm">
      {rg.group_description && <p className="mb-2 text-gray-600">{rg.group_description}</p>}
      <div className="text-xs font-semibold uppercase text-gray-500">Resources ({rg.resources.length})</div>
      <p className="mt-1">
        {rg.resources.map((n, i) => (
          <span key={n}>
            {i > 0 && ", "}
            <Link href={`/resources/detail?name=${encodeURIComponent(n)}`} className="text-brand-700 hover:underline">{n}</Link>
          </span>
        ))}
        {rg.resources.length === 0 && <span className="text-gray-400">none</span>}
      </p>
    </div>
  );
}

export default function ResourceGroupsPage() {
  const [inactive, setInactive] = useState(false);
  const [q, setQ] = useState("");
  const isReviewer = useIsReviewer();
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({ queryKey: ["resource-groups", inactive], queryFn: () => api.resourceGroups(inactive) });

  const rows = (data ?? []).filter(
    (r) => (r.name ?? "").toLowerCase().includes(q.toLowerCase()) || (r.site ?? "").toLowerCase().includes(q.toLowerCase()),
  );

  return (
    <div className="p-8">
      <PageHeader
        title="Resource groups"
        description="A resource group bundles related resources at a site under one administrative unit (production or ITB). Click a row to expand; use the icons to open, edit, or delete."
        action={<LinkButton href="/resource-groups/new">New resource group</LinkButton>}
      />
      <div className="mb-4 flex items-center gap-4">
        <input className={`${input} max-w-md`} placeholder="Search by name or site…" value={q} onChange={(e) => setQ(e.target.value)} />
        <InactiveToggle value={inactive} onChange={setInactive} />
      </div>
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : rows.length === 0 ? (
        <Card><p className="text-sm text-gray-500">No resource groups.</p></Card>
      ) : (
        <DataTable<ResourceGroup>
          rows={rows}
          rowKey={(r) => r.name}
          canDelete={isReviewer}
          columns={[
            { header: "Name", cell: (r) => <span className="font-medium text-navy-900">{r.name}{r.deleted && <span className="ml-2 text-xs text-red-500">(inactive)</span>}</span> },
            { header: "Site", cell: (r) => <span className="text-gray-600">{r.site}</span> },
            { header: "Facility", cell: (r) => <span className="text-gray-500">{r.facility}</span> },
            { header: "Resources", cell: (r) => <span className="text-gray-500">{r.resource_count}</span> },
            { header: "Production", cell: (r) => <span className="text-gray-500">{r.production ? "yes" : "no"}</span> },
          ]}
          expanded={(r) => <ExpandedRG name={r.name} />}
          actions={(r) => ({
            detailHref: `/resource-groups/detail?name=${encodeURIComponent(r.name)}`,
            editHref: `/resource-groups/new?edit=${encodeURIComponent(r.name)}`,
            entityKind: "resource_group",
            name: r.name,
            onChanged: () => qc.invalidateQueries({ queryKey: ["resource-groups"] }),
          })}
        />
      )}
    </div>
  );
}
