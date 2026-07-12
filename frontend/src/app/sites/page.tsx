"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Link from "next/link";
import { api, Site } from "@/lib/api";
import { PageHeader, LinkButton, Card, input } from "@/components/ui";
import { InactiveToggle } from "@/components/BrowseControls";
import { DataTable } from "@/components/DataTable";
import { useIsReviewer } from "@/components/entityActions";

function ExpandedSite({ name }: { name: string }) {
  const { data: s } = useQuery({ queryKey: ["site-detail", name], queryFn: () => api.siteDetail(name) });
  if (!s) return <p className="text-sm text-gray-400">Loading…</p>;
  return (
    <div className="text-sm">
      <p className="text-gray-600">
        {[s.long_name, s.address_line1, s.city, s.state, s.country, s.zipcode].filter(Boolean).join(", ") || "—"}
      </p>
      <div className="mt-2 text-xs font-semibold uppercase text-gray-500">Resource groups ({s.resource_groups.length})</div>
      <p className="mt-1">
        {s.resource_groups.map((n, i) => (
          <span key={n}>
            {i > 0 && ", "}
            <Link href={`/resource-groups/detail?name=${encodeURIComponent(n)}`} className="text-brand-700 hover:underline">{n}</Link>
          </span>
        ))}
        {s.resource_groups.length === 0 && <span className="text-gray-400">none</span>}
      </p>
    </div>
  );
}

export default function SitesPage() {
  const [inactive, setInactive] = useState(false);
  const [q, setQ] = useState("");
  const isReviewer = useIsReviewer();
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({ queryKey: ["sites", inactive], queryFn: () => api.sites(inactive) });

  const rows = (data ?? []).filter((r) => (r.name ?? "").toLowerCase().includes(q.toLowerCase()));

  return (
    <div className="p-8">
      <PageHeader
        title="Sites"
        description="A site is a physical location (campus, data center) belonging to a facility. Click a row to expand; use the icons to open, edit, or delete."
        action={<LinkButton href="/sites/new">New site</LinkButton>}
      />
      <div className="mb-4 flex items-center gap-4">
        <input className={`${input} max-w-md`} placeholder="Search by name…" value={q} onChange={(e) => setQ(e.target.value)} />
        <InactiveToggle value={inactive} onChange={setInactive} />
      </div>
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : rows.length === 0 ? (
        <Card><p className="text-sm text-gray-500">No sites.</p></Card>
      ) : (
        <DataTable<Site>
          rows={rows}
          rowKey={(r) => r.name}
          canDelete={isReviewer}
          columns={[
            { header: "Name", cell: (r) => <span className="font-medium text-navy-900">{r.name}{r.deleted && <span className="ml-2 text-xs text-red-500">(inactive)</span>}</span> },
            { header: "Facility", cell: (r) => <span className="text-gray-600">{r.facility}</span> },
            { header: "City", cell: (r) => <span className="text-gray-500">{r.city}</span> },
            { header: "Country", cell: (r) => <span className="text-gray-500">{r.country}</span> },
          ]}
          expanded={(r) => <ExpandedSite name={r.name} />}
          actions={(r) => ({
            detailHref: `/sites/detail?name=${encodeURIComponent(r.name)}`,
            editHref: `/sites/new?edit=${encodeURIComponent(r.name)}`,
            entityKind: "site",
            name: r.name,
            onChanged: () => qc.invalidateQueries({ queryKey: ["sites"] }),
          })}
        />
      )}
    </div>
  );
}
