"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import { PageHeader, Card, input, btn } from "@/components/ui";
import { DowntimesTable } from "@/components/DowntimesTable";

export default function DowntimesPage() {
  const [q, setQ] = useState("");
  const { data, isLoading } = useQuery({ queryKey: ["downtimes"], queryFn: () => api.downtimes() });
  const { data: session } = useQuery({ queryKey: ["me"], queryFn: api.auth.me, retry: false });
  const canManage = !!session?.user?.id;

  const rows = (data ?? []).filter(
    (d) =>
      (d.resource ?? "").toLowerCase().includes(q.toLowerCase()) ||
      (d.resource_group ?? "").toLowerCase().includes(q.toLowerCase()),
  );

  return (
    <div className="p-8">
      <PageHeader
        title="Downtimes"
        description="Scheduled and unscheduled downtimes across resources. Downtimes also appear on each resource's detail page."
        action={
          canManage ? (
            <Link href="/downtimes/new" className={btn}>
              Register downtime
            </Link>
          ) : undefined
        }
      />
      <input
        className={`${input} mb-4 max-w-md`}
        placeholder="Filter by resource or resource group…"
        value={q}
        onChange={(e) => setQ(e.target.value)}
      />
      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : (
        <Card>
          <DowntimesTable downtimes={rows.slice(0, 1000)} showResource canManage={canManage} />
          {rows.length > 1000 && (
            <p className="mt-2 text-xs text-gray-400">Showing first 1000 of {rows.length}.</p>
          )}
        </Card>
      )}
    </div>
  );
}
