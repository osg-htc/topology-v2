"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { api, ContactReplacement } from "@/lib/api";
import { PageHeader, Card } from "@/components/ui";

const STATUS_BADGE: Record<string, string> = {
  pending: "bg-amber-100 text-amber-800",
  approved: "bg-green-100 text-green-800",
  rejected: "bg-gray-100 text-gray-500",
  withdrawn: "bg-gray-100 text-gray-500",
};

function SlotLabel({ r }: { r: ContactReplacement }) {
  const href =
    r.entity_kind === "resource"
      ? `/resources/detail?name=${encodeURIComponent(r.entity_name)}`
      : r.entity_kind === "resource_group"
        ? `/resource-groups/detail?name=${encodeURIComponent(r.entity_name)}`
        : r.entity_kind === "site"
          ? `/sites/detail?name=${encodeURIComponent(r.entity_name)}`
          : `/facilities/detail?name=${encodeURIComponent(r.entity_name)}`;
  return (
    <span>
      <span className="text-gray-500">{r.contact_type}</span> ({r.rank}) on{" "}
      <Link href={href} className="text-brand-700 hover:underline">
        {r.entity_name}
      </Link>
    </span>
  );
}

export default function ReplacementsPage() {
  const qc = useQueryClient();
  const { data: incoming } = useQuery({ queryKey: ["replacements", "incoming"], queryFn: api.contactReplacements.incoming });
  const { data: mine } = useQuery({ queryKey: ["replacements", "mine"], queryFn: api.contactReplacements.mine });

  const act = async (fn: Promise<unknown>) => {
    try {
      await fn;
      qc.invalidateQueries({ queryKey: ["replacements"] });
    } catch (e) {
      alert(String(e));
    }
  };

  return (
    <div className="p-8">
      <PageHeader
        title="Contact hand-offs"
        description="Requests for someone to take over a contact role. You can approve requests to replace you; you can withdraw requests you made."
      />

      <Card className="mb-6">
        <h3 className="mb-3 text-sm font-semibold text-gray-700">Requests to replace you</h3>
        {(incoming ?? []).length === 0 ? (
          <p className="text-sm text-gray-400">Nothing waiting on you.</p>
        ) : (
          <ul className="space-y-2">
            {(incoming ?? []).map((r) => (
              <li key={r.id} className="flex items-center justify-between border-b border-gray-100 pb-2 text-sm">
                <span>
                  <span className="font-medium text-navy-900">{r.requester_name}</span> wants to take over{" "}
                  <SlotLabel r={r} />
                </span>
                <span className="flex gap-2">
                  <button
                    className="rounded bg-green-600 px-2 py-1 text-xs text-white hover:bg-green-700"
                    onClick={() => act(api.contactReplacements.approve(r.id))}
                  >
                    Approve
                  </button>
                  <button
                    className="rounded border border-gray-300 px-2 py-1 text-xs text-gray-600 hover:bg-gray-50"
                    onClick={() => act(api.contactReplacements.reject(r.id))}
                  >
                    Reject
                  </button>
                </span>
              </li>
            ))}
          </ul>
        )}
      </Card>

      <Card>
        <h3 className="mb-3 text-sm font-semibold text-gray-700">Requests you made</h3>
        {(mine ?? []).length === 0 ? (
          <p className="text-sm text-gray-400">You haven’t requested any hand-offs.</p>
        ) : (
          <ul className="space-y-2">
            {(mine ?? []).map((r) => (
              <li key={r.id} className="flex items-center justify-between border-b border-gray-100 pb-2 text-sm">
                <span>
                  Take over <SlotLabel r={r} />
                  {r.incumbent_name && <span className="text-gray-400"> — currently {r.incumbent_name}</span>}
                </span>
                <span className="flex items-center gap-2">
                  <span className={`rounded-full px-2 py-0.5 text-xs ${STATUS_BADGE[r.status] ?? ""}`}>{r.status}</span>
                  {r.status === "pending" && (
                    <button
                      className="text-xs text-gray-500 hover:underline"
                      onClick={() => act(api.contactReplacements.withdraw(r.id))}
                    >
                      withdraw
                    </button>
                  )}
                </span>
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  );
}
