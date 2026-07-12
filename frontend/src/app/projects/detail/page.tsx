"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card } from "@/components/ui";
import { DetailField } from "@/components/DetailField";

function ProjectDetail() {
  const name = useSearchParams().get("name") || "";
  const { data: p, isLoading } = useQuery({
    queryKey: ["project", name],
    queryFn: () => api.project(name),
    enabled: !!name,
  });

  if (isLoading) return <div className="p-8 text-gray-400">Loading…</div>;
  if (!p) return <div className="p-8 text-gray-400">Project not found.</div>;

  return (
    <div className="p-8">
      <PageHeader title={p.name} description="Project" />
      <Card className="max-w-2xl">
        <dl className="grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-2">
          <DetailField label="ID" value={p.id} />
          <DetailField label="PI" value={p.pi_name} />
          <DetailField label="Organization" value={p.organization} />
          <DetailField label="Department" value={p.department} />
          <DetailField label="Field of science" value={p.field_of_science} />
          <DetailField label="Field of science ID" value={p.field_of_science_id} />
          <DetailField label="Institution ID" value={p.institution_id} />
          <DetailField
            label="Sponsor"
            value={p.sponsor_name ? `${p.sponsor_type}: ${p.sponsor_name}` : ""}
          />
        </dl>
        {p.description && (
          <div className="mt-4">
            <div className="text-xs font-medium uppercase tracking-wide text-gray-500">
              Description
            </div>
            <p className="mt-1 whitespace-pre-wrap text-sm text-gray-700">{p.description}</p>
          </div>
        )}
      </Card>
    </div>
  );
}

export default function ProjectDetailPage() {
  return (
    <Suspense fallback={null}>
      <ProjectDetail />
    </Suspense>
  );
}
