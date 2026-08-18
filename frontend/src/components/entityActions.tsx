"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";

// useIsReviewer reports whether the current session may review/override
// (manager or administrator).
export function useIsReviewer(): boolean {
  const { data } = useQuery({ queryKey: ["me"], queryFn: api.auth.me, retry: false });
  const role = data?.effective_role;
  return role === "manager" || role === "administrator";
}

function TrashIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="h-4 w-4">
      <path d="M3 6h18M8 6V4a1 1 0 011-1h6a1 1 0 011 1v2m2 0v14a1 1 0 01-1 1H7a1 1 0 01-1-1V6h12z" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

// DeleteButton removes an entity. Under the hood every change goes through the
// change-request workflow, so this files a delete request that a reviewer
// approves — but the user just sees "Delete".
export function DeleteButton({
  entityKind,
  name,
  displayName,
  onDone,
}: {
  entityKind: string;
  name: string;
  displayName?: string;
  onDone?: () => void;
}) {
  const label = displayName ?? name;
  const [msg, setMsg] = useState("");
  const mut = useMutation({
    mutationFn: () =>
      api.proposals.create({
        entity_kind: entityKind,
        operation: "delete",
        target_name: name,
        // A delete carries no payload the backend reads, but the review/list
        // pages summarize a proposal from proposed_state -- for a resource,
        // target_name is now the immutable id, so without this the summary
        // would show a raw number instead of the resource's name.
        proposed_state: displayName ? { name: displayName } : undefined,
        submit: true,
      }),
    onSuccess: () => {
      setMsg("delete requested");
      onDone?.();
    },
    onError: (e) => setMsg(String(e)),
  });
  if (msg) return <span className="text-xs text-gray-400">{msg}</span>;
  return (
    <button
      title="Delete"
      aria-label={`Delete ${label}`}
      className="inline-flex items-center rounded p-1 text-gray-400 hover:bg-red-50 hover:text-red-600 disabled:opacity-50"
      onClick={() => {
        if (confirm(`Delete ${label}? This files a change request a reviewer must approve.`))
          mut.mutate();
      }}
      disabled={mut.isPending}
    >
      <TrashIcon />
    </button>
  );
}
