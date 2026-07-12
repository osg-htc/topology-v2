// StructuredView renders an arbitrary JSON-ish value as a readable, nested
// definition list — never as raw JSON. Used to show a proposal's proposed_state
// to reviewers and submitters.

function labelize(key: string): string {
  // snake_case / camelCase → Title Case-ish, but leave already-capitalized keys.
  if (/^[A-Z]/.test(key)) return key;
  return key
    .replace(/_/g, " ")
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .replace(/^\w/, (c) => c.toUpperCase());
}

function Scalar({ value }: { value: unknown }) {
  if (value === null || value === undefined || value === "")
    return <span className="text-gray-400">—</span>;
  if (typeof value === "boolean")
    return <span className="text-gray-800">{value ? "yes" : "no"}</span>;
  return <span className="whitespace-pre-wrap text-gray-800">{String(value)}</span>;
}

function Node({ value }: { value: unknown }) {
  if (value === null || typeof value !== "object") return <Scalar value={value} />;

  if (Array.isArray(value)) {
    if (value.length === 0) return <span className="text-gray-400">(none)</span>;
    const primitive = value.every((v) => v === null || typeof v !== "object");
    if (primitive) return <Scalar value={value.join(", ")} />;
    return (
      <ul className="ml-3 list-disc space-y-1">
        {value.map((v, i) => (
          <li key={i}>
            <Node value={v} />
          </li>
        ))}
      </ul>
    );
  }

  const entries = Object.entries(value as Record<string, unknown>);
  if (entries.length === 0) return <span className="text-gray-400">(empty)</span>;
  return (
    <dl className="space-y-1.5">
      {entries.map(([k, v]) => (
        <div key={k} className="grid grid-cols-[10rem_1fr] gap-2">
          <dt className="text-xs font-medium uppercase tracking-wide text-gray-500">
            {labelize(k)}
          </dt>
          <dd className="text-sm">
            <Node value={v} />
          </dd>
        </div>
      ))}
    </dl>
  );
}

export function StructuredView({ value }: { value: unknown }) {
  return (
    <div className="text-sm">
      <Node value={value} />
    </div>
  );
}
