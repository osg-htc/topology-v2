"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { PageHeader, LinkButton, Card } from "@/components/ui";
import { ProposalRow, proposalMeta } from "@/components/ProposalRow";

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
            <ProposalRow key={p.id} p={p} meta={proposalMeta(p, "updated")} />
          ))}
        </div>
      )}
    </div>
  );
}
