"use client";

import Link from "next/link";
import { Proposal } from "@/lib/api";
import { StatusBadge } from "@/components/ui";
import { proposalSummary, kindLabel } from "@/lib/proposalSummary";
import { UserLabel } from "@/components/UserLabel";

const OP_COLOR: Record<string, string> = {
  create: "bg-green-100 text-green-800",
  update: "bg-blue-100 text-blue-800",
  delete: "bg-red-100 text-red-800",
};

// ProposalRow is one entry in the "my requests" / review lists. It leads with a
// kind chip and the affected entity name, and lists the concrete changes so a
// reviewer knows what it is without opening it.
export function ProposalRow({ p, meta }: { p: Proposal; meta?: React.ReactNode }) {
  let s;
  try {
    s = proposalSummary(p);
  } catch {
    // Never let one odd payload blank the whole list.
    s = { kind: p.entity_kind, title: p.target_name ?? "—", changes: [`${p.operation} ${p.entity_kind}`] };
  }
  return (
    <Link
      href={`/proposals/view?id=${p.id}`}
      className="block rounded-lg border border-gray-200 bg-white px-4 py-3 hover:border-brand-400"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="rounded-full bg-navy-100 px-2 py-0.5 text-xs font-medium text-navy-800">
              {kindLabel(s.kind)}
            </span>
            <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${OP_COLOR[p.operation] ?? "bg-gray-100 text-gray-600"}`}>
              {p.operation}
            </span>
            <span className="truncate font-medium text-navy-900">{s.title}</span>
          </div>
          <div className="mt-1 text-xs text-gray-500">{s.changes.join(" · ")}</div>
          {meta && <div className="mt-1 text-xs text-gray-400">{meta}</div>}
        </div>
        <StatusBadge status={p.status} />
      </div>
    </Link>
  );
}

// proposalMeta renders the "submitted … by …" line used on the lists.
export function proposalMeta(p: Proposal, verb: string): React.ReactNode {
  return (
    <>
      {verb} {new Date(p.updated_at).toLocaleString()} · by <UserLabel id={p.created_by} />
    </>
  );
}
