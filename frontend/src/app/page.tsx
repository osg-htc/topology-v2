"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { api } from "@/lib/api";
import { Card, PageHeader, StatusBadge, LinkButton } from "@/components/ui";

export default function DashboardPage() {
  const { data, isLoading } = useQuery({ queryKey: ["dashboard"], queryFn: api.dashboard });

  if (isLoading || !data) {
    return <div className="p-8 text-gray-400">Loading…</div>;
  }

  return (
    <div className="p-8">
      <PageHeader
        title="Dashboard"
        action={<LinkButton href="/proposals/new">Register a resource</LinkButton>}
      />

      <SummaryTiles />

      <section className="mb-8">
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-gray-500">
          My resources
        </h2>
        {data.my_resources.length === 0 ? (
          <Card>
            <p className="text-sm text-gray-500">
              You are not listed as a contact on any resources yet.
            </p>
          </Card>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {data.my_resources.map((r) => (
              <Card key={r.id}>
                <div className="flex items-center justify-between">
                  <span className="font-medium text-navy-900">{r.name}</span>
                  <span
                    className={`h-2 w-2 rounded-full ${r.active ? "bg-green-500" : "bg-gray-300"}`}
                    title={r.active ? "active" : "inactive"}
                  />
                </div>
                <div className="mt-1 text-xs text-gray-500">{r.fqdn}</div>
                <div className="mt-2 text-xs text-gray-400">{r.resource_group}</div>
              </Card>
            ))}
          </div>
        )}
      </section>

      <section className="mb-8">
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-gray-500">
          My pending registrations
        </h2>
        <ProposalList proposals={data.pending_registrations} empty="No pending registrations." />
      </section>

      {data.can_review && (
        <section>
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-gray-500">
            Pending approvals
          </h2>
          <ProposalList
            proposals={data.pending_approvals}
            empty="Nothing awaiting your review."
            review
          />
        </section>
      )}
    </div>
  );
}

const TILES: { key: keyof import("@/lib/api").Summary; label: string; href: string }[] = [
  { key: "resources", label: "Resources", href: "/resources" },
  { key: "resource_groups", label: "Resource groups", href: "/resource-groups" },
  { key: "sites", label: "Sites", href: "/sites" },
  { key: "facilities", label: "Facilities", href: "/facilities" },
  { key: "institutions", label: "Institutions", href: "/institutions" },
  { key: "vos", label: "VOs", href: "/resources" },
  { key: "projects", label: "Projects", href: "/resources" },
];

function SummaryTiles() {
  const { data } = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  return (
    <section className="mb-8">
      <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-gray-500">
        All topology
      </h2>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        {TILES.filter((t) => t.key !== "vos" && t.key !== "projects").map((t) => (
          <Link
            key={t.key}
            href={t.href}
            className="rounded-lg border border-gray-200 bg-white p-4 hover:border-brand-400"
          >
            <div className="text-2xl font-bold text-navy-900">{data ? data[t.key] : "—"}</div>
            <div className="text-xs uppercase tracking-wide text-gray-500">{t.label}</div>
          </Link>
        ))}
      </div>
    </section>
  );
}

function ProposalList({
  proposals,
  empty,
  review,
}: {
  proposals: import("@/lib/api").Proposal[];
  empty: string;
  review?: boolean;
}) {
  if (!proposals || proposals.length === 0) {
    return (
      <Card>
        <p className="text-sm text-gray-500">{empty}</p>
      </Card>
    );
  }
  return (
    <div className="space-y-2">
      {proposals.map((p) => (
        <Link
          key={p.id}
          href={`/proposals/view?id=${p.id}`}
          className="flex items-center justify-between rounded-lg border border-gray-200 bg-white px-4 py-3 hover:border-brand-400"
        >
          <div>
            <span className="font-medium text-navy-900">
              {p.operation} {p.entity_kind}
            </span>
            {p.target_name && <span className="ml-2 text-sm text-gray-500">{p.target_name}</span>}
          </div>
          <div className="flex items-center gap-3">
            {review && <span className="text-xs text-gray-400">by {p.created_by.slice(0, 8)}</span>}
            <StatusBadge status={p.status} />
          </div>
        </Link>
      ))}
    </div>
  );
}
