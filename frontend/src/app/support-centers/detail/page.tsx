"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card, LinkButton } from "@/components/ui";
import { DetailField } from "@/components/DetailField";
import { EntityProposalHistory } from "@/components/EntityProposalHistory";

function SupportCenterDetail() {
  const name = useSearchParams().get("name") || "";
  const { data: sc, isLoading } = useQuery({
    queryKey: ["support-center", name],
    queryFn: () => api.supportCenter(name),
    enabled: !!name,
  });

  if (isLoading) return <div className="p-8 text-gray-400">Loading…</div>;
  if (!sc) return <div className="p-8 text-gray-400">Support center not found.</div>;

  const contacts = Object.entries(sc.extra?.Contacts ?? {});

  return (
    <div className="p-8">
      <PageHeader
        title={sc.name}
        description="Support center"
        action={<LinkButton href={`/support-centers/new?edit=${encodeURIComponent(sc.name)}`}>Edit</LinkButton>}
      />
      <div className="grid gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-2">
          <Card>
            <dl className="grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-2">
              <DetailField label="ID" value={sc.id} />
              <DetailField label="Long name" value={sc.long_name} />
              <DetailField label="Community" value={sc.community} />
            </dl>
            {sc.description && (
              <div className="mt-4">
                <div className="text-xs font-medium uppercase tracking-wide text-gray-500">Description</div>
                <p className="mt-1 whitespace-pre-wrap text-sm text-gray-700">{sc.description}</p>
              </div>
            )}
          </Card>
          {contacts.length > 0 && (
            <Card>
              <h3 className="mb-2 text-sm font-semibold text-gray-700">Contacts</h3>
              <dl className="space-y-2 text-sm">
                {contacts.map(([type, people]) => (
                  <div key={type}>
                    <dt className="text-xs font-medium uppercase tracking-wide text-gray-500">{type}</dt>
                    <dd className="text-gray-700">{(people ?? []).map((p) => p.Name).filter(Boolean).join(", ") || "—"}</dd>
                  </div>
                ))}
              </dl>
              <p className="mt-3 text-xs text-gray-400">Contacts are kept as-is when a support center is edited here.</p>
            </Card>
          )}
        </div>
        <div className="space-y-6">
          <Card>
            <h3 className="mb-2 text-sm font-semibold text-gray-700">Resource groups ({sc.resource_groups.length})</h3>
            <ul className="space-y-1 text-sm">
              {sc.resource_groups.map((n) => (
                <li key={n}>
                  <Link href={`/resource-groups/detail?name=${encodeURIComponent(n)}`} className="text-brand-700 hover:underline">{n}</Link>
                </li>
              ))}
              {sc.resource_groups.length === 0 && <li className="text-gray-400">None.</li>}
            </ul>
          </Card>
          <EntityProposalHistory entityKind="support_center" targetName={sc.name} />
        </div>
      </div>
    </div>
  );
}

export default function SupportCenterDetailPage() {
  return (
    <Suspense fallback={null}>
      <SupportCenterDetail />
    </Suspense>
  );
}
