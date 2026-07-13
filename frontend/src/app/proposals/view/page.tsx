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

  const summary = proposalSummary(p);

  return (
    <div className="p-8">
      <PageHeader
        title={`${p.operation} ${kindLabel(p.entity_kind).toLowerCase()}: ${summary.title}`}
        description={summary.changes.join(" · ")}
        action={<StatusBadge status={p.status} />}
      />

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
            {p.operation === "delete" ? (
              <p className="text-sm text-gray-600">
                Proposes deleting <span className="font-medium">{p.target_name}</span>.
              </p>
            ) : (
              <StructuredView value={p.proposed_state} />
            )}
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
              {isCreator && p.status === "draft" && (
                <button
                  className={`${btn} w-full justify-center`}
                  onClick={() => act.mutate(() => api.proposals.submit(id))}
                >
                  Submit for review
                </button>
              )}
              {isCreator && (p.status === "draft" || p.status === "pending") && (
                <button
                  className={`${btnSecondary} w-full justify-center`}
                  onClick={() => act.mutate(() => api.proposals.withdraw(id))}
                >
                  Withdraw
                </button>
              )}
              {isReviewer && p.status === "pending" && (
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
              {!isCreator && !isReviewer && (
                <p className="text-xs text-gray-400">No actions available.</p>
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
