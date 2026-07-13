"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card } from "@/components/ui";
import { DetailField } from "@/components/DetailField";
import { EntityContactsCard } from "@/components/EntityContactsCard";

function SiteDetailView() {
  const name = useSearchParams().get("name") || "";
  const { data: s, isLoading } = useQuery({
    queryKey: ["site-detail", name],
    queryFn: () => api.siteDetail(name),
    enabled: !!name,
  });
  if (isLoading) return <div className="p-8 text-gray-400">Loading…</div>;
  if (!s) return <div className="p-8 text-gray-400">Site not found.</div>;

  return (
    <div className="p-8">
      <PageHeader title={s.name} description="Site" />
      <div className="grid gap-6 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <dl className="grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-2">
            <DetailField label="Long name" value={s.long_name} />
            <DetailField label="City" value={s.city} />
            <DetailField label="State" value={s.state} />
            <DetailField label="Country" value={s.country} />
            <DetailField label="Zipcode" value={s.zipcode} />
            <DetailField label="Address" value={s.address_line1} />
            <DetailField label="Latitude" value={s.latitude} />
            <DetailField label="Longitude" value={s.longitude} />
          </dl>
          <div className="mt-2 text-sm">
            <span className="text-gray-400">Facility: </span>
            <Link href={`/facilities/detail?name=${encodeURIComponent(s.facility)}`} className="text-brand-700 hover:underline">{s.facility}</Link>
          </div>
          {s.description && (
            <div className="mt-4">
              <div className="text-xs font-medium uppercase tracking-wide text-gray-500">Description</div>
              <p className="mt-1 whitespace-pre-wrap text-sm text-gray-700">{s.description}</p>
            </div>
          )}
        </Card>
        <Card>
          <h3 className="mb-2 text-sm font-semibold text-gray-700">
            Resource groups ({s.resource_groups.length})
          </h3>
          <ul className="space-y-1 text-sm">
            {s.resource_groups.map((n) => (
              <li key={n}>
                <Link href={`/resource-groups/detail?name=${encodeURIComponent(n)}`} className="text-brand-700 hover:underline">{n}</Link>
              </li>
            ))}
            {s.resource_groups.length === 0 && <li className="text-gray-400">None.</li>}
          </ul>
        </Card>
      </div>
      <EntityContactsCard entityKind="site" entityName={s.name} contacts={s.contacts} />
    </div>
  );
}

export default function SiteDetailPage() {
  return (
    <Suspense fallback={null}>
      <SiteDetailView />
    </Suspense>
  );
}
