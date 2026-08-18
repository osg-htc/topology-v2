"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api, DashboardResource } from "@/lib/api";
import { PageHeader, LinkButton, Card, input } from "@/components/ui";
import { DataTable } from "@/components/DataTable";
import { useIsReviewer } from "@/components/entityActions";

function ExpandedResource({ id }: { id: number }) {
  const { data: r } = useQuery({
    queryKey: ["resource-detail", id],
    queryFn: () => api.resourceDetail(id),
  });
  if (!r) return <p className="text-sm text-gray-400">Loading…</p>;
  return (
    <div className="grid gap-4 text-sm sm:grid-cols-2">
      <div>
        <div className="text-xs font-semibold uppercase text-gray-500">Contacts</div>
        {(r.contacts ?? []).length === 0 ? (
          <p className="text-amber-600">None — required for a complete registration.</p>
        ) : (
          <ul className="mt-1 space-y-0.5">
            {r.contacts.map((c, i) => (
              <li key={i} className="text-gray-700">
                {c.contact_type} ({c.rank}): {c.name || c.id || "—"}
              </li>
            ))}
          </ul>
        )}
      </div>
      <div>
        <div className="text-xs font-semibold uppercase text-gray-500">Services</div>
        <p className="mt-1 text-gray-700">
          {(r.services ?? []).map((s) => s.name).join(", ") || "—"}
        </p>
        <div className="mt-2 text-xs font-semibold uppercase text-gray-500">Tags / VOs</div>
        <p className="mt-1 text-gray-600">
          {(r.tags ?? []).join(", ") || "no tags"} · VOs: {(r.allowed_vos ?? []).join(", ") || "—"}
        </p>
      </div>
    </div>
  );
}

export default function ResourcesPage() {
  const { data, isLoading } = useQuery({ queryKey: ["resources"], queryFn: api.resources });
  const [q, setQ] = useState("");
  const isReviewer = useIsReviewer();
  const qc = useQueryClient();

  const needle = q.toLowerCase();
  const rows = Object.values(data ?? {})
    .filter(
      (r) =>
        (r.name ?? "").toLowerCase().includes(needle) ||
        (r.fqdn ?? "").toLowerCase().includes(needle),
    )
    .sort((a, b) => (a.name ?? "").localeCompare(b.name ?? ""));

  return (
    <div className="p-8">
      <PageHeader
        title="Resources"
        description="Resources are the individual services and endpoints OSG knows about (a CE, an access point, an XRootD/Pelican origin or cache, …). Click a row to expand; use the icons to open, edit, or delete."
        action={<LinkButton href="/proposals/new">Register a resource</LinkButton>}
      />
      <input
        className={`${input} mb-4 max-w-md`}
        placeholder="Search by name or host name…"
        value={q}
        onChange={(e) => setQ(e.target.value)}
      />
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : rows.length === 0 ? (
        <Card>
          <p className="text-sm text-gray-500">No resources match.</p>
        </Card>
      ) : (
        <DataTable<DashboardResource>
          rows={rows}
          rowKey={(r) => String(r.id)}
          canDelete={isReviewer}
          columns={[
            { header: "Name", sortValue: (r) => r.name, cell: (r) => <span className="font-medium text-navy-900">{r.name}</span> },
            { header: "Host name", sortValue: (r) => r.fqdn, cell: (r) => <span className="text-gray-600">{r.fqdn}</span> },
            { header: "Resource group", sortValue: (r) => r.resource_group, cell: (r) => <span className="text-gray-500">{r.resource_group}</span> },
            {
              header: "Status",
              sortValue: (r) => (r.active ? "active" : "inactive"),
              cell: (r) => (
                <span
                  className={`rounded-full px-2 py-0.5 text-xs ${
                    r.active ? "bg-brand-100 text-brand-800" : "bg-gray-100 text-gray-500"
                  }`}
                >
                  {r.active ? "active" : "inactive"}
                </span>
              ),
            },
          ]}
          expanded={(r) => <ExpandedResource id={r.id} />}
          actions={(r) => ({
            detailHref: `/resources/detail?id=${r.id}`,
            editHref: `/proposals/new?edit=${r.id}`,
            entityKind: "resource",
            name: String(r.id),
            displayName: r.name,
            onChanged: () => qc.invalidateQueries({ queryKey: ["resources"] }),
          })}
        />
      )}
    </div>
  );
}
