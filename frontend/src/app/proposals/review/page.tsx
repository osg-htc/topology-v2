"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { PageHeader, Card } from "@/components/ui";
import { ProposalRow, proposalMeta } from "@/components/ProposalRow";

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
            <ProposalRow key={p.id} p={p} meta={proposalMeta(p, "submitted")} />
          ))}
        </div>
      )}
    </div>
  );
}
