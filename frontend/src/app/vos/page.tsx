"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api, VOListRow } from "@/lib/api";
import { PageHeader, LinkButton, Card, input } from "@/components/ui";
import { DataTable } from "@/components/DataTable";
import { useIsReviewer } from "@/components/entityActions";

function Chip({ children, tone }: { children: React.ReactNode; tone: "red" | "gray" }) {
  const cls = tone === "red" ? "bg-red-100 text-red-700" : "bg-gray-100 text-gray-500";
  return <span className={`ml-2 rounded-full px-2 py-0.5 text-xs font-medium ${cls}`}>{children}</span>;
}

export default function VOsPage() {
  const [q, setQ] = useState("");
  const isReviewer = useIsReviewer();
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({ queryKey: ["vos"], queryFn: api.vos });

  const rows = (data ?? []).filter(
    (r) =>
      r.name.toLowerCase().includes(q.toLowerCase()) ||
      r.long_name.toLowerCase().includes(q.toLowerCase()) ||
      r.community.toLowerCase().includes(q.toLowerCase()),
  );

  return (
    <div className="p-8">
      <PageHeader
        title="Virtual organizations"
        description="The VOs that resources can allow and projects can be sponsored by. Changes go through the same review as any other change request. Click a row to expand; use the icons to open, edit, or delete."
        action={<LinkButton href="/vos/new">New VO</LinkButton>}
      />
      <div className="mb-4">
        <input className={`${input} max-w-md`} placeholder="Search by name, long name, or community…" value={q} onChange={(e) => setQ(e.target.value)} />
      </div>
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : rows.length === 0 ? (
        <Card><p className="text-sm text-gray-500">No VOs.</p></Card>
      ) : (
        <DataTable<VOListRow>
          rows={rows}
          rowKey={(r) => r.name}
          canDelete={isReviewer}
          columns={[
            {
              header: "Name",
              sortValue: (r) => r.name,
              cell: (r) => (
                <span className="font-medium text-navy-900">
                  {r.name}
                  {r.disable && <Chip tone="red">disabled</Chip>}
                  {!r.disable && !r.active && <Chip tone="gray">inactive</Chip>}
                </span>
              ),
            },
            { header: "Long name", sortValue: (r) => r.long_name, cell: (r) => <span className="text-gray-600">{r.long_name || "—"}</span> },
            { header: "Community", sortValue: (r) => r.community, cell: (r) => <span className="line-clamp-2 text-gray-500">{r.community || "—"}</span> },
            { header: "Parent", sortValue: (r) => r.parent_vo, cell: (r) => <span className="text-gray-500">{r.parent_vo || "—"}</span> },
          ]}
          actions={(r) => ({
            detailHref: `/vos/detail?name=${encodeURIComponent(r.name)}`,
            editHref: `/vos/new?edit=${encodeURIComponent(r.name)}`,
            entityKind: "vo",
            name: r.name,
            onChanged: () => qc.invalidateQueries({ queryKey: ["vos"] }),
          })}
        />
      )}
    </div>
  );
}
