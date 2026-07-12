"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { PageHeader, LinkButton, input } from "@/components/ui";

export default function ResourcesPage() {
  const { data, isLoading } = useQuery({ queryKey: ["resources"], queryFn: api.resources });
  const [q, setQ] = useState("");

  const needle = q.toLowerCase();
  const rows = Object.values(data ?? {})
    .filter(
      (r) =>
        (r.name ?? "").toLowerCase().includes(needle) ||
        (r.fqdn ?? "").toLowerCase().includes(needle),
    )
    .sort((a, b) => (a.name ?? "").localeCompare(b.name ?? ""));

  return (
    <div className="p-8">
      <PageHeader
        title="Resources"
        action={<LinkButton href="/proposals/new">Register a resource</LinkButton>}
      />
      <input
        className={`${input} mb-4 max-w-md`}
        placeholder="Search by name or FQDN…"
        value={q}
        onChange={(e) => setQ(e.target.value)}
      />
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white">
          <table className="min-w-full text-sm">
            <thead className="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500">
              <tr>
                <th className="px-4 py-2">Name</th>
                <th className="px-4 py-2">FQDN</th>
                <th className="px-4 py-2">Resource group</th>
                <th className="px-4 py-2">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {rows.map((r) => (
                <tr key={r.id} className="hover:bg-gray-50">
                  <td className="px-4 py-2 font-medium text-navy-900">{r.name}</td>
                  <td className="px-4 py-2 text-gray-600">{r.fqdn}</td>
                  <td className="px-4 py-2 text-gray-500">{r.resource_group}</td>
                  <td className="px-4 py-2">
                    <span
                      className={`rounded-full px-2 py-0.5 text-xs ${
                        r.active ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-500"
                      }`}
                    >
                      {r.active ? "active" : "inactive"}
                    </span>
                  </td>
                </tr>
              ))}
              {rows.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-4 py-6 text-center text-gray-400">
                    No resources match.
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
