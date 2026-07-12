"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { PageHeader, Card } from "@/components/ui";

export default function AuditPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ["audit"],
    queryFn: api.audit,
    retry: false,
  });

  return (
    <div className="p-8">
      <PageHeader title="Audit log" />
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : error ? (
        <Card>
          <p className="text-sm text-red-600">Manager or administrator role required.</p>
        </Card>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white">
          <table className="min-w-full text-sm">
            <thead className="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500">
              <tr>
                <th className="px-4 py-2">When</th>
                <th className="px-4 py-2">Actor</th>
                <th className="px-4 py-2">Action</th>
                <th className="px-4 py-2">Entity</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {(data ?? []).map((e) => (
                <tr key={e.id}>
                  <td className="px-4 py-2 text-gray-500">
                    {new Date(e.created_at).toLocaleString()}
                  </td>
                  <td className="px-4 py-2 text-gray-500">{(e.actor_user_id ?? "").slice(0, 8)}</td>
                  <td className="px-4 py-2 font-medium text-navy-900">{e.action}</td>
                  <td className="px-4 py-2 text-gray-500">
                    {e.entity_kind}
                    {e.entity_id ? ` / ${e.entity_id}` : ""}
                  </td>
                </tr>
              ))}
              {(!data || data.length === 0) && (
                <tr>
                  <td colSpan={4} className="px-4 py-6 text-center text-gray-400">
                    No audit entries yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
