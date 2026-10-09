"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api, SupportCenter } from "@/lib/api";
import { PageHeader, LinkButton, Card, input } from "@/components/ui";
import { DataTable } from "@/components/DataTable";
import { useIsReviewer } from "@/components/entityActions";

export default function SupportCentersPage() {
  const [q, setQ] = useState("");
  const isReviewer = useIsReviewer();
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({ queryKey: ["support-centers"], queryFn: api.supportCenters });

  const rows = (data ?? []).filter(
    (r) =>
      r.name.toLowerCase().includes(q.toLowerCase()) ||
      r.long_name.toLowerCase().includes(q.toLowerCase()) ||
      r.community.toLowerCase().includes(q.toLowerCase()),
  );

  return (
    <div className="p-8">
      <PageHeader
        title="Support centers"
        description="The support organizations a resource group can name. Changes go through the same review as any other change request. Click a row to expand; use the icons to open, edit, or delete."
        action={<LinkButton href="/support-centers/new">New support center</LinkButton>}
      />
      <div className="mb-4">
        <input className={`${input} max-w-md`} placeholder="Search by name, long name, or community…" value={q} onChange={(e) => setQ(e.target.value)} />
      </div>
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : rows.length === 0 ? (
        <Card><p className="text-sm text-gray-500">No support centers.</p></Card>
      ) : (
        <DataTable<SupportCenter>
          rows={rows}
          rowKey={(r) => r.name}
          canDelete={isReviewer}
          columns={[
            { header: "Name", sortValue: (r) => r.name, cell: (r) => <span className="font-medium text-navy-900">{r.name}</span> },
            { header: "Long name", sortValue: (r) => r.long_name, cell: (r) => <span className="text-gray-600">{r.long_name || "—"}</span> },
            { header: "Community", sortValue: (r) => r.community, cell: (r) => <span className="text-gray-500">{r.community || "—"}</span> },
            { header: "Resource groups", sortValue: (r) => r.resource_group_count, cell: (r) => <span className="text-gray-500">{r.resource_group_count}</span> },
          ]}
          actions={(r) => ({
            detailHref: `/support-centers/detail?name=${encodeURIComponent(r.name)}`,
            editHref: `/support-centers/new?edit=${encodeURIComponent(r.name)}`,
            entityKind: "support_center",
            name: r.name,
            onChanged: () => qc.invalidateQueries({ queryKey: ["support-centers"] }),
          })}
        />
      )}
    </div>
  );
}
