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

// ProposeDeleteButton files a delete proposal for an entity. Since all changes
// go through the proposal workflow, this proposes (pending) rather than deleting
// outright; a reviewer approves it to apply the soft-delete.
export function ProposeDeleteButton({
  entityKind,
  name,
  onDone,
}: {
  entityKind: string;
  name: string;
  onDone?: () => void;
}) {
  const [msg, setMsg] = useState("");
  const mut = useMutation({
    mutationFn: () =>
      api.proposals.create({
        entity_kind: entityKind,
        operation: "delete",
        target_name: name,
        submit: true,
      }),
    onSuccess: () => {
      setMsg("delete proposed");
      onDone?.();
    },
    onError: (e) => setMsg(String(e)),
  });
  return (
    <span className="text-xs">
      {msg ? (
        <span className="text-gray-400">{msg}</span>
      ) : (
        <button
          className="text-red-600 hover:underline"
          onClick={() => {
            if (confirm(`Propose deletion of ${name}? A reviewer must approve it.`)) mut.mutate();
          }}
          disabled={mut.isPending}
        >
          Propose delete
        </button>
      )}
    </span>
  );
}
