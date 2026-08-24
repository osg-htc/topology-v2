"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Suspense, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { api } from "@/lib/api";
import { PageHeader, Card, StatusBadge, btn, btnSecondary, input } from "@/components/ui";
import { StructuredView } from "@/components/StructuredView";
import { UserLabel } from "@/components/UserLabel";
import { proposalSummary, kindLabel } from "@/lib/proposalSummary";

// editFormHref maps a proposal to the structured form that can edit it, passing
// enough context to prefill. Only resource groups currently prefill (via name).
function editFormHref(kind: string, state: unknown): string | null {
  const s = (state ?? {}) as Record<string, unknown>;
  switch (kind) {
    case "resource":
      return `/proposals/new${s.resource_group ? `?rg=${encodeURIComponent(String(s.resource_group))}` : ""}`;
    case "resource_group":
      return "/resource-groups/new";
    case "site":
      return "/sites/new";
    case "facility":
      return "/facilities/new";
    case "project":
      return "/projects/new";
    default:
      return null;
  }
}

function ProposalView() {
  const params = useSearchParams();
  const id = params.get("id") || "";
  const qc = useQueryClient();

  const { data: session } = useQuery({ queryKey: ["me"], queryFn: api.auth.me });
  const { data: p, isLoading } = useQuery({
    queryKey: ["proposal", id],
    queryFn: () => api.proposals.get(id),
    enabled: !!id,
  });

  const [note, setNote] = useState("");
  const [msg, setMsg] = useState("");

  const refresh = () => {
    qc.invalidateQueries({ queryKey: ["proposal", id] });
    qc.invalidateQueries({ queryKey: ["dashboard"] });
    // Also refresh the review queue and "my requests" lists so a
    // rejected/approved/withdrawn request drops off them immediately.
    qc.invalidateQueries({ queryKey: ["proposals"] });
  };

  const act = useMutation({
    mutationFn: async (fn: () => Promise<unknown>) => fn(),
    onSuccess: () => {
      setMsg("Done.");
      refresh();
    },
    onError: (e) => setMsg(String(e)),
  });

  if (isLoading || !p) return <div className="p-8 text-gray-400">Loading…</div>;

  const role = session?.effective_role;
  const isReviewer = role === "manager" || role === "administrator";
  const isCreator = session?.user.id === p.created_by;
  const editable = p.status === "draft" || p.status === "pending";
  // Route "edit in form" to the matching structured create/edit form.
  const editHref = editFormHref(p.entity_kind, p.proposed_state);

  // Drives which Actions buttons render below -- and, when none of them do,
  // whether that's because the proposal is already decided (applied/
  // rejected/withdrawn) or because this viewer just has no role here. Either
  // way the Actions card must say something, not render empty.
  const canSubmit = isCreator && p.status === "draft";
  const canWithdraw = isCreator && (p.status === "draft" || p.status === "pending");
  const canReview = isReviewer && p.status === "pending";
  const hasActions = canSubmit || canWithdraw || canReview;
  const decided = p.status !== "draft" && p.status !== "pending";

  const summary = proposalSummary(p);

  return (
    <div className="p-8">
      <PageHeader
        title={`${p.operation} ${kindLabel(p.entity_kind).toLowerCase()}: ${summary.title}`}
        description={summary.changes.join(" · ")}
        action={<StatusBadge status={p.status} />}
      />

      {p.base_stale && (
        <div className="mb-4 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">
          This proposal's base is out of date — the underlying record has
          changed since this proposal was created. Approving it may
          overwrite that change.
        </div>
      )}

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2 space-y-4">
          <Card>
            <div className="mb-3 flex items-center justify-between">
              <h3 className="text-sm font-semibold text-gray-700">
                Proposed change{" "}
                <span className="font-normal text-gray-400">(schema v{p.schema_version})</span>
              </h3>
              {editable && isCreator && editHref && (
                <Link href={editHref} className="text-xs text-brand-600 hover:underline">
                  Edit in form
                </Link>
              )}
            </div>
            <div className="grid gap-4 md:grid-cols-2">
              <div className="min-w-0">
                <h4 className="mb-1 text-xs font-semibold uppercase tracking-wide text-gray-400">Before</h4>
                {p.operation === "create" ? (
                  <p className="text-sm text-gray-400">New — nothing existed yet.</p>
                ) : p.base_version ? (
                  <StructuredView value={p.base_version} other={p.proposed_state} />
                ) : (
                  <p className="text-sm text-gray-400">Not available.</p>
                )}
              </div>
              <div className="min-w-0">
                <h4 className="mb-1 text-xs font-semibold uppercase tracking-wide text-gray-400">Proposed</h4>
                {p.operation === "delete" ? (
                  <p className="text-sm text-gray-600">
                    Proposes deleting{" "}
                    <span className="font-medium">
                      {(p.proposed_state as { name?: string } | null)?.name ?? p.target_name}
                    </span>
                    .
                  </p>
                ) : (
                  <StructuredView value={p.proposed_state} other={p.base_version ?? undefined} />
                )}
              </div>
            </div>
          </Card>

          {p.revisions && p.revisions.length > 0 && (
            <Card>
              <h3 className="mb-2 text-sm font-semibold text-gray-700">Revision history</h3>
              <ul className="space-y-1 text-xs text-gray-500">
                {p.revisions.map((r) => (
                  <li key={r.revision_no} className="flex justify-between">
                    <span>
                      #{r.revision_no} by <UserLabel id={r.edited_by} />
                      {r.note ? ` — ${r.note}` : ""}
                    </span>
                    <span>{new Date(r.created_at).toLocaleString()}</span>
                  </li>
                ))}
              </ul>
            </Card>
          )}
        </div>

        <div className="space-y-3">
          <Card>
            <h3 className="mb-3 text-sm font-semibold text-gray-700">Actions</h3>
            <div className="space-y-2">
              {canSubmit && (
                <button
                  className={`${btn} w-full justify-center`}
                  onClick={() => act.mutate(() => api.proposals.submit(id))}
                >
                  Submit for review
                </button>
              )}
              {canWithdraw && (
                <button
                  className={`${btnSecondary} w-full justify-center`}
                  onClick={() => act.mutate(() => api.proposals.withdraw(id))}
                >
                  Withdraw
                </button>
              )}
              {canReview && (
                <>
                  <button
                    className={`${btn} w-full justify-center`}
                    onClick={() => act.mutate(() => api.proposals.approve(id))}
                  >
                    Approve &amp; apply
                  </button>
                  <input
                    className={input}
                    placeholder="Reason (for reject)"
                    value={note}
                    onChange={(e) => setNote(e.target.value)}
                  />
                  <button
                    className={`${btnSecondary} w-full justify-center`}
                    onClick={() => act.mutate(() => api.proposals.reject(id, note))}
                  >
                    Reject
                  </button>
                </>
              )}
              {!hasActions && (
                <p className="text-xs text-gray-400">
                  {decided
                    ? `This proposal has already been ${p.status} — no further action needed.`
                    : "No actions available."}
                </p>
              )}
            </div>
            {msg && <p className="mt-3 text-xs text-gray-500">{msg}</p>}
          </Card>
          {p.review_note && (
            <Card>
              <h3 className="mb-1 text-sm font-semibold text-gray-700">Review note</h3>
              <p className="text-sm text-gray-600">{p.review_note}</p>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}

export default function ProposalViewPage() {
  return (
    <Suspense fallback={null}>
      <ProposalView />
    </Suspense>
  );
}
