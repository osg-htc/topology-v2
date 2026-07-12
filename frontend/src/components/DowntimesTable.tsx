import { Downtime } from "@/lib/api";

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
}: {
  downtimes: Downtime[];
  showResource?: boolean;
}) {
  if (!downtimes || downtimes.length === 0) {
    return <p className="text-sm text-gray-400">No downtimes.</p>;
  }
  const sorted = [...downtimes].sort((a, b) => (a.start_time < b.start_time ? 1 : -1));
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full text-sm">
        <thead className="text-left text-xs uppercase tracking-wide text-gray-400">
          <tr>
            <th className="py-1 pr-4">When</th>
            {showResource && <th className="py-1 pr-4">Resource</th>}
            <th className="py-1 pr-4">Class</th>
            <th className="py-1 pr-4">Severity</th>
            <th className="py-1 pr-4">Services</th>
            <th className="py-1">Start → End</th>
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
                <td className="py-1 text-xs text-gray-500">
                  {d.start_time} → {d.end_time}
                  {d.description && <div className="text-gray-400">{d.description}</div>}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
