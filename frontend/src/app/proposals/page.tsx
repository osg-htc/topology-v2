"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { api } from "@/lib/api";
import { PageHeader, StatusBadge, LinkButton, Card } from "@/components/ui";

export default function MyProposalsPage() {
  const { data, isLoading } = useQuery({ queryKey: ["proposals", "mine"], queryFn: api.proposals.mine });

  return (
    <div className="p-8">
      <PageHeader
        title="My change requests"
        description="Change requests you've submitted to register or modify topology entities. A manager or administrator reviews and approves each one before it takes effect."
        action={<LinkButton href="/proposals/new">Register a resource</LinkButton>}
      />
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : !data || data.length === 0 ? (
        <Card>
          <p className="text-sm text-gray-500">You have no change requests yet.</p>
        </Card>
      ) : (
        <div className="space-y-2">
          {data.map((p) => (
            <Link
              key={p.id}
              href={`/proposals/view?id=${p.id}`}
              className="flex items-center justify-between rounded-lg border border-gray-200 bg-white px-4 py-3 hover:border-brand-400"
            >
              <div>
                <span className="font-medium text-navy-900">
                  {p.operation} {p.entity_kind}
                </span>
                {p.target_name && (
                  <span className="ml-2 text-sm text-gray-500">{p.target_name}</span>
                )}
                <div className="text-xs text-gray-400">
                  updated {new Date(p.updated_at).toLocaleString()}
                </div>
              </div>
              <StatusBadge status={p.status} />
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
