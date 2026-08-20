"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { api, Proposal } from "@/lib/api";
import { Card, StatusBadge } from "@/components/ui";
import { UserLabel } from "@/components/UserLabel";

const OP_COLOR: Record<string, string> = {
  create: "bg-green-100 text-green-800",
  update: "bg-blue-100 text-blue-800",
  delete: "bg-red-100 text-red-800",
};

function formatShort(iso: string): string {
  return new Date(iso).toLocaleString(undefined, { dateStyle: "short", timeStyle: "short" });
}

// EntityProposalHistory shows the proposals that make up a given entity's
// actual edit history (pending or applied only -- see
// ListProposalsByEntity). Deliberately not the general ProposalRow: this
// lives in a narrow sidebar, already knows what entity/page it's on, so it
// drops the entity title and change-summary line and shows one name per
// actor instead of "Display name (username)".
export function EntityProposalHistory({
  entityKind,
  targetName,
}: {
  entityKind: string;
  targetName: string;
}) {
  const { data, isLoading } = useQuery({
    queryKey: ["entity-proposals", entityKind, targetName],
    queryFn: () => api.proposals.byEntity(entityKind, targetName),
    enabled: !!targetName,
  });

  return (
    <Card>
      <h3 className="mb-2 text-sm font-semibold text-gray-700">Edit history</h3>
      {isLoading ? (
        <p className="text-sm text-gray-400">Loading…</p>
      ) : !data || data.length === 0 ? (
        <p className="text-sm text-gray-500">No proposals have targeted this yet.</p>
      ) : (
        <div className="max-h-96 space-y-1.5 overflow-y-auto pr-1">
          {data.map((p) => (
            <HistoryRow key={p.id} p={p} />
          ))}
        </div>
      )}
    </Card>
  );
}

function HistoryRow({ p }: { p: Proposal }) {
  return (
    <Link
      href={`/proposals/view?id=${p.id}`}
      className="block rounded-md border border-gray-200 bg-white px-2.5 py-1.5 text-xs hover:border-brand-400"
    >
      <div className="flex items-center justify-between gap-1.5">
        <span className="font-medium text-navy-900">
          <UserLabel id={p.created_by} compact />
        </span>
        <span className="flex items-center gap-1.5">
          <span className={`rounded-full px-1.5 py-0.5 font-medium ${OP_COLOR[p.operation] ?? "bg-gray-100 text-gray-600"}`}>
            {p.operation}
          </span>
          <StatusBadge status={p.status} />
        </span>
      </div>
      <div className="mt-1 text-gray-500">Proposed {formatShort(p.created_at)}</div>
      {p.status !== "pending" && (
        <div className="text-gray-500">
          {p.status} {formatShort(p.updated_at)} by <UserLabel id={p.assigned_reviewer ?? ""} compact />
        </div>
      )}
    </Link>
  );
}
