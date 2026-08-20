"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";

// UserLabel renders an actor as "Display name (username)", with the immutable
// user id available on hover (title) — used wherever a change was made "by"
// someone. Labels are cached per id by React Query. Pass `compact` in tight
// spaces (e.g. a sidebar list) to show only one name — display name if set,
// else the username — instead of both.
export function UserLabel({ id, compact = false }: { id: string; compact?: boolean }) {
  const { data } = useQuery({
    queryKey: ["user-label", id],
    queryFn: () => api.userLabels([id]),
    enabled: !!id,
    staleTime: 5 * 60_000,
  });
  if (!id) return <span className="text-gray-400">—</span>;
  const l = data?.[0];
  if (!l) {
    return (
      <span className="text-gray-400" title={id}>
        {id.slice(0, 8)}…
      </span>
    );
  }
  if (compact) {
    return (
      <span title={l.id} className="cursor-help">
        {l.display_name || l.username || "(no name)"}
      </span>
    );
  }
  return (
    <span title={l.id} className="cursor-help">
      {l.display_name || "(no name)"}
      {l.username ? <span className="text-gray-400"> ({l.username})</span> : null}
    </span>
  );
}
