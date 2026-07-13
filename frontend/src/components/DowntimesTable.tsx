"use client";

import Link from "next/link";
import { Downtime, api } from "@/lib/api";

// timeframe buckets a downtime relative to now.
function timeframe(d: Downtime): "past" | "current" | "future" {
  const now = Date.now();
  const start = Date.parse(d.start_time.replace(/ ([+-]\d{4})$/, " GMT$1"));
  const end = Date.parse(d.end_time.replace(/ ([+-]\d{4})$/, " GMT$1"));
  if (!isNaN(end) && end < now) return "past";
  if (!isNaN(start) && start > now) return "future";
  return "current";
}

const BADGE: Record<string, string> = {
  current: "bg-red-100 text-red-800",
  future: "bg-amber-100 text-amber-800",
  past: "bg-gray-100 text-gray-500",
};

export function DowntimesTable({
  downtimes,
  showResource = false,
  canManage = false,
  cap = false,
}: {
  downtimes: Downtime[];
  showResource?: boolean;
  canManage?: boolean;
  // cap limits the table to ~5 rows of height with a scrollbar and a sticky
  // header, so a long downtime history doesn't dominate a detail page.
  cap?: boolean;
}) {
  const removeDowntime = async (d: Downtime) => {
    if (!confirm(`Propose removal of this downtime on ${d.resource}?`)) return;
    try {
      const res = await api.proposals.create({
        entity_kind: "downtime",
        operation: "delete",
        target_name: String(d.id),
        submit: true,
        proposed_state: { resource: d.resource, class: d.class },
      });
      window.location.href = `/proposals/view?id=${res.id}`;
    } catch (e) {
      alert(String(e));
    }
  };
  if (!downtimes || downtimes.length === 0) {
    return <p className="text-sm text-gray-400">No downtimes.</p>;
  }
  // Current first, then future, then past; within a bucket, most recently
  // started first. Expired downtimes sink to the bottom.
  const bucketRank = { current: 0, future: 1, past: 2 };
  const startMs = (d: Downtime) => Date.parse(d.start_time.replace(/ ([+-]\d{4})$/, " GMT$1")) || 0;
  const sorted = [...downtimes].sort((a, b) => {
    const ra = bucketRank[timeframe(a)];
    const rb = bucketRank[timeframe(b)];
    if (ra !== rb) return ra - rb;
    return startMs(b) - startMs(a);
  });
  return (
    <div className={`overflow-x-auto${cap ? " max-h-60 overflow-y-auto" : ""}`}>
      <table className="min-w-full text-sm">
        <thead className={`text-left text-xs uppercase tracking-wide text-gray-400${cap ? " sticky top-0 bg-white" : ""}`}>
          <tr>
            <th className="py-1 pr-4">When</th>
            {showResource && <th className="py-1 pr-4">Resource</th>}
            <th className="py-1 pr-4">Class</th>
            <th className="py-1 pr-4">Severity</th>
            <th className="py-1 pr-4">Services</th>
            <th className="py-1 pr-4">Start → End</th>
            {canManage && <th className="py-1">Actions</th>}
          </tr>
        </thead>
        <tbody>
          {sorted.map((d) => {
            const tf = timeframe(d);
            return (
              <tr key={d.id} className="border-t border-gray-100 align-top">
                <td className="py-1 pr-4">
                  <span className={`rounded-full px-2 py-0.5 text-xs ${BADGE[tf]}`}>{tf}</span>
                </td>
                {showResource && <td className="py-1 pr-4 text-gray-700">{d.resource}</td>}
                <td className="py-1 pr-4 text-gray-600">{d.class}</td>
                <td className="py-1 pr-4 text-gray-600">{d.severity}</td>
                <td className="py-1 pr-4 text-gray-500">{(d.services ?? []).join(", ") || "—"}</td>
                <td className="py-1 pr-4 text-xs text-gray-500">
                  {d.start_time} → {d.end_time}
                  {d.description && <div className="text-gray-400">{d.description}</div>}
                </td>
                {canManage && (
                  <td className="py-1 text-xs whitespace-nowrap">
                    <Link
                      href={`/downtimes/new?edit=${d.id}`}
                      className="text-brand-700 hover:underline"
                    >
                      edit
                    </Link>
                    <button
                      type="button"
                      onClick={() => removeDowntime(d)}
                      className="ml-3 text-red-600 hover:underline"
                    >
                      delete
                    </button>
                  </td>
                )}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
