"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card } from "@/components/ui";
import { DetailField } from "@/components/DetailField";
import { DowntimesTable } from "@/components/DowntimesTable";

function RGDetailView() {
  const name = useSearchParams().get("name") || "";
  const { data: rg, isLoading } = useQuery({
    queryKey: ["rg-detail", name],
    queryFn: () => api.resourceGroupDetail(name),
    enabled: !!name,
  });
  const { data: downtimes } = useQuery({
    queryKey: ["downtimes", "rg", name],
    queryFn: () => api.downtimes({ rg: name }),
    enabled: !!name,
  });
  if (isLoading) return <div className="p-8 text-gray-400">Loading…</div>;
  if (!rg) return <div className="p-8 text-gray-400">Resource group not found.</div>;

  return (
    <div className="p-8">
      <PageHeader title={rg.name} description="Resource group" />
      <div className="grid gap-6 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <dl className="grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-2">
            <DetailField label="Group ID" value={rg.group_id} />
            <DetailField label="Production" value={rg.production ? "yes" : "no"} />
            <DetailField label="Support center" value={rg.support_center} />
            <DetailField
              label="Site"
              value={undefined}
            />
          </dl>
          <div className="mt-2 text-sm">
            <span className="text-gray-400">Site: </span>
            <Link href={`/sites/detail?name=${encodeURIComponent(rg.site)}`} className="text-brand-700 hover:underline">{rg.site}</Link>
            <span className="text-gray-400"> · Facility: </span>
            <Link href={`/facilities/detail?name=${encodeURIComponent(rg.facility)}`} className="text-brand-700 hover:underline">{rg.facility}</Link>
          </div>
          {rg.group_description && (
            <div className="mt-4">
              <div className="text-xs font-medium uppercase tracking-wide text-gray-500">Description</div>
              <p className="mt-1 whitespace-pre-wrap text-sm text-gray-700">{rg.group_description}</p>
            </div>
          )}
        </Card>
        <Card>
          <h3 className="mb-2 text-sm font-semibold text-gray-700">
            Resources ({rg.resources.length})
          </h3>
          <ul className="space-y-1 text-sm">
            {rg.resources.map((n) => (
              <li key={n}>
                <Link href={`/resources/detail?name=${encodeURIComponent(n)}`} className="text-brand-700 hover:underline">
                  {n}
                </Link>
              </li>
            ))}
            {rg.resources.length === 0 && <li className="text-gray-400">None.</li>}
          </ul>
        </Card>
      </div>
      <Card className="mt-6">
        <h3 className="mb-3 text-sm font-semibold text-gray-700">Downtimes</h3>
        <DowntimesTable downtimes={downtimes ?? []} showResource />
      </Card>
    </div>
  );
}

export default function RGDetailPage() {
  return (
    <Suspense fallback={null}>
      <RGDetailView />
    </Suspense>
  );
}
