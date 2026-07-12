"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { api } from "@/lib/api";
import { PageHeader, Card, StatusBadge } from "@/components/ui";

export default function ReviewQueuePage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ["proposals", "pending"],
    queryFn: api.proposals.pending,
    retry: false,
  });

  return (
    <div className="p-8">
      <PageHeader
        title="Review queue"
        description="Change requests awaiting review. Approve to apply the change, or reject with a reason."
      />
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : error ? (
        <Card>
          <p className="text-sm text-red-600">
            You need the manager or administrator role to review change requests.
          </p>
        </Card>
      ) : !data || data.length === 0 ? (
        <Card>
          <p className="text-sm text-gray-500">Nothing awaiting review.</p>
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
                {p.target_name && <span className="ml-2 text-sm text-gray-500">{p.target_name}</span>}
                <div className="text-xs text-gray-400">by {p.created_by.slice(0, 8)}</div>
              </div>
              <StatusBadge status={p.status} />
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
