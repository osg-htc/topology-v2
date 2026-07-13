"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card } from "@/components/ui";
import { DetailField } from "@/components/DetailField";
import { EntityContactsCard } from "@/components/EntityContactsCard";

function FacilityDetailView() {
  const name = useSearchParams().get("name") || "";
  const { data: f, isLoading } = useQuery({
    queryKey: ["facility-detail", name],
    queryFn: () => api.facilityDetail(name),
    enabled: !!name,
  });
  if (isLoading) return <div className="p-8 text-gray-400">Loading…</div>;
  if (!f) return <div className="p-8 text-gray-400">Facility not found.</div>;

  return (
    <div className="p-8">
      <PageHeader title={f.name} description="Facility" />
      <div className="grid gap-6 lg:grid-cols-3">
        <Card className="lg:col-span-2">
          <dl className="grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-2">
            <DetailField label="Facility ID" value={f.facility_id} />
            <DetailField label="Institution ID" value={f.institution_id} />
          </dl>
        </Card>
        <Card>
          <h3 className="mb-2 text-sm font-semibold text-gray-700">Sites ({f.sites.length})</h3>
          <ul className="space-y-1 text-sm">
            {f.sites.map((n) => (
              <li key={n}>
                <Link href={`/sites/detail?name=${encodeURIComponent(n)}`} className="text-brand-700 hover:underline">{n}</Link>
              </li>
            ))}
            {f.sites.length === 0 && <li className="text-gray-400">None.</li>}
          </ul>
        </Card>
      </div>
      <EntityContactsCard entityKind="facility" entityName={f.name} contacts={f.contacts} />
    </div>
  );
}

export default function FacilityDetailPage() {
  return (
    <Suspense fallback={null}>
      <FacilityDetailView />
    </Suspense>
  );
}
