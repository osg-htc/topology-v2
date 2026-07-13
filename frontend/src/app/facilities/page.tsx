"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import Link from "next/link";
import { api, Facility } from "@/lib/api";
import { PageHeader, LinkButton, Card, input } from "@/components/ui";
import { InactiveToggle } from "@/components/BrowseControls";
import { DataTable } from "@/components/DataTable";
import { useIsReviewer } from "@/components/entityActions";

function ExpandedFacility({ name }: { name: string }) {
  const { data: f } = useQuery({ queryKey: ["facility-detail", name], queryFn: () => api.facilityDetail(name) });
  if (!f) return <p className="text-sm text-gray-400">Loading…</p>;
  return (
    <div className="text-sm">
      <div className="text-xs font-semibold uppercase text-gray-500">Sites ({f.sites.length})</div>
      <p className="mt-1">
        {f.sites.map((n, i) => (
          <span key={n}>
            {i > 0 && ", "}
            <Link href={`/sites/detail?name=${encodeURIComponent(n)}`} className="text-brand-700 hover:underline">{n}</Link>
          </span>
        ))}
        {f.sites.length === 0 && <span className="text-gray-400">none</span>}
      </p>
    </div>
  );
}

export default function FacilitiesPage() {
  const [inactive, setInactive] = useState(false);
  const [q, setQ] = useState("");
  const isReviewer = useIsReviewer();
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({ queryKey: ["facilities", inactive], queryFn: () => api.facilities(inactive) });

  const rows = (data ?? []).filter((r) => (r.name ?? "").toLowerCase().includes(q.toLowerCase()));

  return (
    <div className="p-8">
      <PageHeader
        title="Facilities"
        description="A facility is the top-level organization (a university or lab) that owns one or more sites and links to an institution. Click a row to expand; use the icons to open, edit, or delete."
        action={<LinkButton href="/facilities/new">New facility</LinkButton>}
      />
      <div className="mb-4 flex items-center gap-4">
        <input className={`${input} max-w-md`} placeholder="Search by name…" value={q} onChange={(e) => setQ(e.target.value)} />
        <InactiveToggle value={inactive} onChange={setInactive} />
      </div>
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : rows.length === 0 ? (
        <Card><p className="text-sm text-gray-500">No facilities.</p></Card>
      ) : (
        <DataTable<Facility>
          rows={rows}
          rowKey={(r) => r.name}
          canDelete={isReviewer}
          columns={[
            { header: "Name", sortValue: (r) => r.name, cell: (r) => <span className="font-medium text-navy-900">{r.name}{r.deleted && <span className="ml-2 text-xs text-red-500">(inactive)</span>}</span> },
            { header: "Institution ID", sortValue: (r) => r.institution_id, cell: (r) => <span className="text-xs text-gray-500">{r.institution_id || "—"}</span> },
            { header: "Sites", sortValue: (r) => r.site_count, cell: (r) => <span className="text-gray-500">{r.site_count}</span> },
          ]}
          expanded={(r) => <ExpandedFacility name={r.name} />}
          actions={(r) => ({
            detailHref: `/facilities/detail?name=${encodeURIComponent(r.name)}`,
            editHref: `/facilities/new?edit=${encodeURIComponent(r.name)}`,
            entityKind: "facility",
            name: r.name,
            onChanged: () => qc.invalidateQueries({ queryKey: ["facilities"] }),
          })}
        />
      )}
    </div>
  );
}
